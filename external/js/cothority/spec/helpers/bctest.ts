import { Log } from "../../src";
import { ByzCoinRPC, IStorage, LocalCache } from "../../src/byzcoin";
import { Darc, Rule } from "../../src/darc";
import { RosterWSConnection } from "../../src/network";
import { StatusRPC } from "../../src/status";
import { StatusRequest, StatusResponse } from "../../src/status/proto";
import { TransactionBuilder } from "../../src/v2/byzcoin";
import { CoinContract, DarcInst } from "../../src/v2/byzcoin/contracts";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes, stopConodes } from "../support/conondes";

export class BCTest {

    static async singleton(): Promise<BCTest> {
        if (BCTest.bct === undefined) {
            BCTest.bct = await BCTest.init();
        }

        return BCTest.bct;
    }
    private static bct: BCTest | undefined;

    private static async init(): Promise<BCTest> {
        Log.lvl = 1;

        let usesDocker = true;
        try {
            const ws = new RosterWSConnection(ROSTER, StatusRPC.serviceName);
            ws.setParallel(1);
            await ws.send(new StatusRequest(), StatusResponse);
            Log.warn("Using already running nodes for test!");
            usesDocker = false;
        } catch (e) {
            await startConodes();
        }

        const roster = ROSTER.slice(0, 4);
        const cache = new LocalCache();
        const genesis = ByzCoinRPC.makeGenesisDarc([SIGNER], roster, "initial");
        [CoinContract.ruleFetch, CoinContract.ruleMint, CoinContract.ruleSpawn, CoinContract.ruleStore,
            CoinContract.ruleTransfer]
            .forEach((rule) => genesis.addIdentity(rule, SIGNER, Rule.OR));
        const rpc = await ByzCoinRPC.newByzCoinRPC(roster, genesis, BLOCK_INTERVAL, cache);
        const tx = new TransactionBuilder(rpc);
        const genesisInst = await DarcInst.retrieve(rpc, genesis.getBaseID());
        return new BCTest(cache, genesis, genesisInst, rpc, tx, usesDocker);
    }

    private constructor(
        public cache: IStorage,
        public genesis: Darc,
        public genesisInst: DarcInst,
        public rpc: ByzCoinRPC,
        public tx: TransactionBuilder,
        public usesDocker: boolean,
    ) {
    }

    async shutdown() {
        if (this.usesDocker) {
            return stopConodes();
        }
    }
}
