import * as LND from 'types/generated/lnd_pb';
import { Lightning } from 'types/generated/lnd_pb_service';
import { DEV_MACAROON } from 'config';
import GrpcClient from './grpc';

/**
 * An API wrapper to communicate with the LND node via GRPC
 */
class LndApi {
  _meta = {
    'X-Grpc-Backend': 'lnd',
    macaroon: DEV_MACAROON,
  };

  private _grpc: GrpcClient;

  constructor(grpc: GrpcClient) {
    this._grpc = grpc;
  }

  /**
   * call the LND `GetInfo` RPC and return the response
   */
  async getInfo(): Promise<LND.GetInfoResponse.AsObject> {
    const req = new LND.GetInfoRequest();
    const res = await this._grpc.request(Lightning.GetInfo, req, this._meta);
    return res.toObject();
  }

  /**
   * call the LND `ListChannels` RPC and return the response
   */
  async listChannels(): Promise<LND.ListChannelsResponse.AsObject> {
    const req = new LND.ListChannelsRequest();
    const res = await this._grpc.request(Lightning.ListChannels, req, this._meta);
    return res.toObject();
  }
}

export default LndApi;