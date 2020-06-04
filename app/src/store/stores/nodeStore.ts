import { action, observable, runInAction, toJS } from 'mobx';
import Big from 'big.js';
import { Store } from 'store';
import { Wallet } from '../models';

type NodeChain = 'bitcoin' | 'litecoin';
type NodeNetwork = 'mainnet' | 'testnet' | 'regtest';

export default class NodeStore {
  private _store: Store;

  /** the pubkey of the LND node */
  @observable pubkey = '';
  /** the alias of the LND node */
  @observable alias = '';
  /** the chain that the LND node is connected to */
  @observable chain: NodeChain = 'bitcoin';
  /** the network that the LND node is connected to */
  @observable network: NodeNetwork = 'mainnet';
  /** the channel and wallet balances */
  @observable wallet: Wallet = new Wallet();

  constructor(store: Store) {
    this._store = store;
  }

  /**
   * fetch wallet balances from the LND RPC
   */
  @action.bound
  async fetchInfo() {
    this._store.log.info('fetching node info');
    try {
      const info = await this._store.api.lnd.getInfo();
      runInAction('getInfoContinuation', () => {
        this.pubkey = info.identityPubkey;
        this.alias = info.alias;
        if (info.chainsList && info.chainsList[0]) {
          this.chain = info.chainsList[0].chain as NodeChain;
          this.network = info.chainsList[0].network as NodeNetwork;
        }
        this._store.log.info('updated nodeStore info', toJS(this));
      });
    } catch (error) {
      this._store.uiStore.handleError(error, 'Unable to fetch node info');
    }
  }

  /**
   * fetch wallet balances from the LND RPC
   */
  @action.bound
  async fetchBalances() {
    this._store.log.info('fetching node balances');
    try {
      const offChain = await this._store.api.lnd.channelBalance();
      const onChain = await this._store.api.lnd.walletBalance();
      runInAction('fetchBalancesContinuation', () => {
        this.wallet.channelBalance = Big(offChain.balance);
        this.wallet.walletBalance = Big(onChain.totalBalance);
        this._store.log.info('updated nodeStore.wallet', toJS(this.wallet));
      });
    } catch (error) {
      this._store.uiStore.handleError(error, 'Unable to fetch balances');
    }
  }
}