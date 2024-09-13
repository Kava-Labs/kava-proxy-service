package postgres

import (
	"context"
	"fmt"
	"github.com/kava-labs/kava-proxy-service/config"
	"math"
	"strings"
	"time"
)

const (
	PartitionBaseTableName = "proxied_request_metrics"
)

// PartitionPeriod represents a single postgres partitioned
// table from a starting point (inclusive of that point in time)
// to an end point (exclusive of that point in time)
type PartitionPeriod struct {
	TableName            string
	InclusiveStartPeriod time.Time
	ExclusiveEndPeriod   time.Time
}

// daysInMonth returns the number of days in a month
func daysInMonth(t time.Time) int {
	y, m, _ := t.Date()
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// PartitionsForPeriod attempts to generate the partitions
// to create when prefilling numDaysToPrefill, returning the list of
// of partitions and error (if any)
func PartitionsForPeriod(start time.Time, numDaysToPrefill int) ([]PartitionPeriod, error) {
	var partitionPeriods []PartitionPeriod
	// check function constraints needed to ensure expected behavior
	if numDaysToPrefill > config.MaxMetricPartitioningPrefillPeriodDays {
		return partitionPeriods, fmt.Errorf("more than %d prefill days specified %d", config.MaxMetricPartitioningPrefillPeriodDays, numDaysToPrefill)
	}

	currentYear, currentMonth, currentDay := start.Date()

	daysInCurrentMonth := daysInMonth(start)

	// add one to include the current day
	newDaysRemainingInCurrentMonth := daysInCurrentMonth - currentDay + 1

	// generate partitions for current month
	totalPartitionsToGenerate := numDaysToPrefill

	partitionsToGenerateForCurrentMonth := int(math.Min(float64(newDaysRemainingInCurrentMonth), float64(numDaysToPrefill)))

	// generate partitions for current month
	for partitionIndex := 0; partitionsToGenerateForCurrentMonth > 0; partitionIndex++ {
		partitionPeriod := PartitionPeriod{
			TableName:            fmt.Sprintf("%s_year_%d_month_%d_day_%d", PartitionBaseTableName, currentYear, currentMonth, currentDay+partitionIndex),
			InclusiveStartPeriod: start.Add(time.Duration(partitionIndex) * 24 * time.Hour).Truncate(24 * time.Hour),
			ExclusiveEndPeriod:   start.Add(time.Duration(partitionIndex+1) * 24 * time.Hour).Truncate(24 * time.Hour),
		}

		partitionPeriods = append(partitionPeriods, partitionPeriod)

		partitionsToGenerateForCurrentMonth--
	}

	// check to see if we need to create any partitions for the
	// upcoming month
	if totalPartitionsToGenerate > newDaysRemainingInCurrentMonth {
		futureMonth := start.Add(time.Hour * 24 * time.Duration(newDaysRemainingInCurrentMonth+1))

		nextYear, nextMonth, nextDay := futureMonth.Date()

		// on function entry we assert that pre-fill days won't
		// overflow more than two unique months
		// to generate partitions for
		partitionsToGenerateForFutureMonth := totalPartitionsToGenerate - newDaysRemainingInCurrentMonth

		// generate partitions for future month
		for partitionIndex := 0; partitionsToGenerateForFutureMonth > 0; partitionIndex++ {
			partitionPeriod := PartitionPeriod{
				TableName:            fmt.Sprintf("%s_year%d_month%d_day%d", PartitionBaseTableName, nextYear, nextMonth, nextDay+partitionIndex),
				InclusiveStartPeriod: futureMonth.Add(time.Duration(partitionIndex) * 24 * time.Hour).Truncate(24 * time.Hour),
				ExclusiveEndPeriod:   futureMonth.Add(time.Duration(partitionIndex+1) * 24 * time.Hour).Truncate(24 * time.Hour),
			}

			partitionPeriods = append(partitionPeriods, partitionPeriod)

			partitionsToGenerateForFutureMonth--
		}
	}

	return partitionPeriods, nil
}

// partition attempts to create (idempotently) future partitions
// for storing proxied request metrics, returning error (if any)
func (c *Client) Partition(prefillPeriodDays int) error {
	// calculate partition name and ranges to create
	partitionsToCreate, err := PartitionsForPeriod(time.Now(), prefillPeriodDays)

	if err != nil {
		return err
	}

	c.logger.Trace().Msg(fmt.Sprintf("partitionsToCreate %+v", partitionsToCreate))

	// create partition for each of those days
	for _, partitionToCreate := range partitionsToCreate {
		// do below in a transaction to allow retries
		// each run of the routine to smooth any over transient issues
		// such as dropped database connection or rolling service updates
		// and support safe concurrency of multiple instances of the service
		// attempting to create partitions
		// https://go.dev/doc/database/execute-transactions
		tx, err := c.db.BeginTx(context.Background(), nil)

		if err != nil {
			c.logger.Error().Msg(fmt.Sprintf("error %s beginning transaction for partition %+v", err, partitionToCreate))

			continue
		}

		// check to see if partition already exists
		_, err = tx.Exec(fmt.Sprintf("select * from %s limit 1;", partitionToCreate.TableName))

		if err != nil {
			if !strings.Contains(err.Error(), "42P01") {
				c.logger.Error().Msg(fmt.Sprintf("error %s querying for partition %+v", err, partitionToCreate))

				tx.Rollback()

				continue
			}

			// else error indicates table doesn't exist so safe for us to create it
			createTableStatement := fmt.Sprintf(`
				CREATE TABLE IF NOT EXISTS %s
					(LIKE proxied_request_metrics INCLUDING DEFAULTS INCLUDING CONSTRAINTS);
			`, partitionToCreate.TableName)
			_, err = c.db.Exec(createTableStatement)

			if err != nil {
				c.logger.Debug().Msg(fmt.Sprintf("error %s creating partition %+v using statement %s", err, partitionToCreate, createTableStatement))

				err = tx.Rollback()

				if err != nil {
					c.logger.Error().Msg(fmt.Sprintf("error %s rolling back statement %s", err, createTableStatement))
				}

				continue
			}

			// attach partitions to main table
			attachPartitionStatement := fmt.Sprintf(`
			ALTER TABLE proxied_request_metrics ATTACH PARTITION %s
			FOR VALUES FROM ('%s') TO ('%s');
			`, partitionToCreate.TableName, partitionToCreate.InclusiveStartPeriod.Format("2006-01-02 15:04:05"), partitionToCreate.ExclusiveEndPeriod.Format("2006-01-02 15:04:05"))
			_, err = c.db.Exec(attachPartitionStatement)

			if err != nil {
				c.logger.Debug().Msg(fmt.Sprintf("error %s attaching partition %+v using statement %s", err,
					partitionToCreate, attachPartitionStatement))

				err = tx.Rollback()

				if err != nil {
					c.logger.Error().Msg(fmt.Sprintf("error %s rolling back statement %s", err, attachPartitionStatement))
				}

				continue
			}

			err = tx.Commit()

			if err != nil {
				c.logger.Error().Msg(fmt.Sprintf("error %s committing transaction to create partition %+v", err, partitionToCreate))

				continue
			}

			c.logger.Trace().Msg(fmt.Sprintf("created partition %+v", partitionToCreate))

			continue
		} else {
			// table exists, no need to create it
			c.logger.Trace().Msg(fmt.Sprintf("not creating table for partition %+v as it already exists", partitionToCreate))

			// but check if it is attached
			partitionIsAttachedQuery := fmt.Sprintf(`
		SELECT
			nmsp_parent.nspname AS parent_schema,
			parent.relname      AS parent,
			nmsp_child.nspname  AS child_schema,
			child.relname       AS child
		FROM pg_inherits
			JOIN pg_class parent            ON pg_inherits.inhparent = parent.oid
			JOIN pg_class child             ON pg_inherits.inhrelid   = child.oid
			JOIN pg_namespace nmsp_parent   ON nmsp_parent.oid  = parent.relnamespace
			JOIN pg_namespace nmsp_child    ON nmsp_child.oid   = child.relnamespace
		WHERE parent.relname='proxied_request_metrics' and child.relname='%s';`, partitionToCreate.TableName)
			result, err := c.db.Query(partitionIsAttachedQuery)

			if err != nil {
				c.logger.Error().Msg(fmt.Sprintf("error %s querying %s to see if partition %+v is already attached", err, partitionIsAttachedQuery, partitionToCreate))

				continue
			}

			if !result.Next() {
				c.logger.Trace().Msg(fmt.Sprintf("attaching created but dangling partition %+v", partitionToCreate))
				// table is not attached, attach it
				attachPartitionStatement := fmt.Sprintf(`
				ALTER TABLE proxied_request_metrics ATTACH PARTITION %s
				FOR VALUES FROM ('%s') TO ('%s');
				`, partitionToCreate.TableName, partitionToCreate.InclusiveStartPeriod.Format("2006-01-02 15:04:05"), partitionToCreate.ExclusiveEndPeriod.Format("2006-01-02 15:04:05"))
				_, err = c.db.Exec(attachPartitionStatement)

				if err != nil {
					c.logger.Debug().Msg(fmt.Sprintf("error %s attaching partition %+v using statement %s", err,
						partitionToCreate, attachPartitionStatement))

					err = tx.Rollback()

					if err != nil {
						c.logger.Error().Msg(fmt.Sprintf("error %s rolling back statement %s", err, attachPartitionStatement))
					}

					continue
				}

				err = tx.Commit()

				if err != nil {
					c.logger.Error().Msg(fmt.Sprintf("error %s committing transaction to create partition %+v", err, partitionToCreate))

					continue
				}

				c.logger.Trace().Msg(fmt.Sprintf("created partition %+v", partitionToCreate))

				continue
			}

			result.Close()

			c.logger.Trace().Msg(fmt.Sprintf("not attaching partition %+v as it is already attached", partitionToCreate))

			err = tx.Commit()

			if err != nil {
				c.logger.Error().Msg(fmt.Sprintf("error %s committing empty transaction for already created partition %+v", err, partitionToCreate))
			}

			continue
		}
	}

	return nil
}
