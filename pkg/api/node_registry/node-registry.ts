
export interface NodeInfo {
  connectors: { [key: string]: ConnectorInfo };
  ips: string[];
  presets: string[];
  publicKey: string;
}

export interface ConnectorInfo {
  protocol: string;
  data: string;
}

export interface RegisterRequest {
  id: string;
  info: NodeInfo;
}

export interface RegisterResponse {
}

export interface GetRequest {
  id: string;
}

export interface GetResponse {
  found: boolean;
  info: NodeInfo;
}
