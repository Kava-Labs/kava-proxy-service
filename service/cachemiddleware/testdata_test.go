package cachemiddleware_test

type testHttpRequestResponse struct {
	RequestBody  string
	ResponseBody string
}

type testResponseName string

const (
	TestResponse_Web3ClientVersion         testResponseName = "web3_clientVersion"
	TestResponse_EthBlockByNumber_Specific testResponseName = "eth_getBlockByNumber"
	TestResponse_EthBlockByNumber_Latest   testResponseName = "eth_getBlockByNumber/latest"
	TestResponse_EthBlockByNumber_Future   testResponseName = "eth_getBlockByNumber/future"
	TestResponse_EthBlockByNumber_Error    testResponseName = "eth_getBlockByNumber/error"
)

// testResponses is a map of testing json-rpc responses. These are copied from
// real requests to the Kava evm.
var testResponses = map[testResponseName]testHttpRequestResponse{
	TestResponse_Web3ClientVersion: {
		RequestBody: `{
			"jsonrpc":"2.0",
			"method":"web3_clientVersion",
			"params":[],
			"id":1
		}`,
		ResponseBody: `{
			"jsonrpc": "2.0",
			"id": 1,
			"result": "Version dev ()\nCompiled at  using Go go1.20.3 (amd64)"
		}`,
	},
	TestResponse_EthBlockByNumber_Specific: {
		RequestBody: `{
			"jsonrpc":"2.0",
			"method":"eth_getBlockByNumber",
			"params":[
				"0x1b4", 
				true
			],
			"id":1
		}`,
		ResponseBody: `{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
			  "difficulty": "0x0",
			  "extraData": "0x",
			  "gasLimit": "0x1312d00",
			  "gasUsed": "0x1afc2",
			  "hash": "0xcc6963a6d025ec2dad24373fbd5f2c3ab75b51ccb31f049682e2001b1a20322f",
			  "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			  "miner": "0x7f73862f0672c066c3f6b4330a736479f0345cd7",
			  "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
			  "nonce": "0x0000000000000000",
			  "number": "0x1b4",
			  "parentHash": "0xd313a81b36d717e4ce67cb7d8f6560158bef9a25f8a4e1b63475050a4181102c",
			  "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
			  "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
			  "size": "0x2431",
			  "stateRoot": "0x20197ba04e30d29a58b508b752d41f0614ceb8d47d2ea2544ff64a6490327625",
			  "timestamp": "0x628e85a0",
			  "totalDifficulty": "0x0",
			  "transactions": [],
			  "transactionsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
			  "uncles": []
			}
		  }`,
	},
	TestResponse_EthBlockByNumber_Latest: {
		RequestBody: `{
			"jsonrpc":"2.0",
			"method":"eth_getBlockByNumber",
			"params":[
				"latest", 
				true
			],
			"id":1
		}`,
		ResponseBody: `{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
			  "difficulty": "0x0",
			  "extraData": "0x",
			  "gasLimit": "0x1312d00",
			  "gasUsed": "0xffea13",
			  "hash": "0xe1cbbd4ba91685ce6c3fe51f2a64cc29d81beee2926d803f7f9ba59fba42fb43",
			  "logsBloom": "0x9030082000000200200000040002300000000000040008000000000000800100800001000040002008000400800080000880021000000802000002001000080000060040800000000140ac09010020200000a0100000000010000200040000004400048042040018008843800a00080080004000000c00001000001108800288000000014080008000001a80000040020400900000020000800201500044210880001000080040c10c081000000000400000040400000480000021001040000000000002000812002084000700430010000000410008028800c20000100020000001000001210040080010000000010240082880400000000000208000004008",
			  "miner": "0xb21adc77c091742783061ab15a0bd1c27efc7a81",
			  "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
			  "nonce": "0x0000000000000000",
			  "number": "0x49be70",
			  "parentHash": "0x7e30cc8b5f6208d0c07d7964930a8dc5d111e4f5830744121beeb5d028c8332d",
			  "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
			  "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
			  "size": "0x2364",
			  "stateRoot": "0x992ed1d2a240baf82ac21bd1d143f000181f9d15f0b8f1b03ee33b0b704d32ce",
			  "timestamp": "0x6465223a",
			  "totalDifficulty": "0x0",
			  "transactions": [
				{
				  "blockHash": "0xe1cbbd4ba91685ce6c3fe51f2a64cc29d81beee2926d803f7f9ba59fba42fb43",
				  "blockNumber": "0x49be70",
				  "from": "0xbfa2f9018a41a5419d38bf3e11e8651e998037c5",
				  "gas": "0x895440",
				  "gasPrice": "0x1e",
				  "hash": "0x58dc15c522cce394167619c3d80ed6c7645db4b43b4759d2808e7468be6808cf",
				  "input": "0xfdb5a03e",
				  "nonce": "0x5e3c6",
				  "to": "0x109f3289665a8f034e2cacdbcfb678cabe09f1d5",
				  "transactionIndex": "0x0",
				  "value": "0x0",
				  "type": "0x0",
				  "chainId": "0x8ae",
				  "v": "0x1180",
				  "r": "0x89834b451fd30d4c66e35b14af33d1759541f9758fac889a1fb47dab7759db64",
				  "s": "0x4511c49cc3ab4dcddcb8e427b3c3b3208c947899e32e461b694efa01d44b2d23"
				},
				{
				  "blockHash": "0xe1cbbd4ba91685ce6c3fe51f2a64cc29d81beee2926d803f7f9ba59fba42fb43",
				  "blockNumber": "0x49be70",
				  "from": "0xd479f39e2d2cf61a6708d2a68b245ed04c10683d",
				  "gas": "0x4c4b40",
				  "gasPrice": "0x37",
				  "hash": "0xf71127d678911732c7654d4fcff7b7b690a552e2b1652d5b981a03b93d64bee0",
				  "input": "0xfdb5a03e",
				  "nonce": "0x71e2a",
				  "to": "0xbc50b9f7f8a4ac5cfbb02d214239033dd5a35527",
				  "transactionIndex": "0x1",
				  "value": "0x0",
				  "type": "0x0",
				  "chainId": "0x8ae",
				  "v": "0x117f",
				  "r": "0x19e8dfcca57dbad7359b4cd48347f184a08ab0b939ccf3a6978ec4bedf6507c3",
				  "s": "0xcd02607f329646b02fff56a211a5442f6c5b13fba6bc01a39edcf6c1d6bdd1e"
				},
				{
				  "blockHash": "0xe1cbbd4ba91685ce6c3fe51f2a64cc29d81beee2926d803f7f9ba59fba42fb43",
				  "blockNumber": "0x49be70",
				  "from": "0xbfa2f9018a41a5419d38bf3e11e8651e998037c5",
				  "gas": "0x895440",
				  "gasPrice": "0x1e",
				  "hash": "0x2ee916a9b0732d7badd7d9f5bb7d933bb5344bb672f73bf741b922ff9a9d2252",
				  "input": "0xfdb5a03e",
				  "nonce": "0x5e3c7",
				  "to": "0x738114fc34d7b0d33f13d2b5c3d44484ec85c7f1",
				  "transactionIndex": "0x2",
				  "value": "0x0",
				  "type": "0x0",
				  "chainId": "0x8ae",
				  "v": "0x1180",
				  "r": "0x81dfe4351c448ccc6a5f6f2a2f866ad023df7843da56c38fb19dd0d5d90e22de",
				  "s": "0x1d770062f304732b4fa5544bd12bd4356f9a853d66bbdef547eab034b142897a"
				},
				{
				  "blockHash": "0xe1cbbd4ba91685ce6c3fe51f2a64cc29d81beee2926d803f7f9ba59fba42fb43",
				  "blockNumber": "0x49be70",
				  "from": "0x07f92d445d1fa59059b50fb664a7633b86db1152",
				  "gas": "0x989680",
				  "gasPrice": "0x3c",
				  "hash": "0x9cb2439b6d4784d58118d3facb9beba77dc80fa4c998b281a4cf46e944298c1f",
				  "input": "0xfdb5a03e",
				  "nonce": "0x58885",
				  "to": "0xefa8952a4ab8b210a5f1dd2a378ed3d1200cf64b",
				  "transactionIndex": "0x3",
				  "value": "0x0",
				  "type": "0x0",
				  "chainId": "0x8ae",
				  "v": "0x117f",
				  "r": "0x515203b1e08dc06fa7a1fa4e029775a233aa6f9af419185d0b6b8a6407d27eb5",
				  "s": "0x52984774ebce17affe5f842ad930605381e5ee146074aa1adc171eb4cc128270"
				},
				{
				  "blockHash": "0xe1cbbd4ba91685ce6c3fe51f2a64cc29d81beee2926d803f7f9ba59fba42fb43",
				  "blockNumber": "0x49be70",
				  "from": "0x6d4f641c7f86c5c76182066b7bc1023dfe51c8f0",
				  "gas": "0x424f3",
				  "gasPrice": "0x3b9aca00",
				  "hash": "0x102a60dcd2333651c5f13e53db5ffad994e3fd4d74d99eb1355562da0b1b4d8d",
				  "input": "0xabe50f1900000000000000000000000000000000000000000000010f0cf064dd592000000000000000000000000000000000000000000000000000000000000000000000",
				  "nonce": "0x376",
				  "to": "0x2911c3a3b497af71aacbb9b1e9fd3ee5d50f959d",
				  "transactionIndex": "0x4",
				  "value": "0x0",
				  "type": "0x0",
				  "chainId": "0x8ae",
				  "v": "0x1180",
				  "r": "0x1890b8bf21b7a50f4a55568535dcff47d89a257e780e51e419c237af943f2afe",
				  "s": "0x313b04a2e0ea400623fbcfd22af91ebd2593c3fdbd29fbe1004e81e54430770a"
				}
			  ],
			  "transactionsRoot": "0xfa3bae7d2ee5eff10fe2ff44840d31755c56eb0dfd0827ebc5ad21ac628020d3",
			  "uncles": []
			}
		}`,
	},
	TestResponse_EthBlockByNumber_Future: {
		RequestBody: `{
			"jsonrpc":"2.0",
			"method":"eth_getBlockByNumber",
			"params":[
				"0x59be70", 
				true
			],
			"id":1
		}`,
		ResponseBody: `{
			"jsonrpc": "2.0",
			"id": 1,
			"result": null
		}`,
	},
	TestResponse_EthBlockByNumber_Error: {
		RequestBody: `{
			"jsonrpc":"2.0",
			"method":"eth_getBlockByNumber",
			"params":[
				oops
			],
			"id":1
		}`,
		ResponseBody: `{
			"jsonrpc": "2.0",
			"id": null,
			"error": {
			  "code": -32700,
			  "message": "parse error"
			}
		  }`,
	},
}
