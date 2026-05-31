import { api } from "./api";

export interface Gateway {
  id: number;
  name: string;
  description?: string;
  proxy: string;
  register: boolean;
  username?: string;
  has_password: boolean;
  realm?: string;
  from_user?: string;
  from_domain?: string;
  transport: string;
  expire_seconds?: number;
  retry_seconds?: number;
  caller_id_in_from?: boolean;
  extra_params?: Record<string, string>;
  enabled: boolean;
  is_active: boolean;
  register_status: "unknown" | "registered" | "trying" | "failed" | "noreg" | "down";
  register_status_at?: string;
  created_at: string;
  updated_at: string;
}

export interface GatewayInput {
  name?: string;
  description?: string;
  proxy?: string;
  register?: boolean;
  username?: string;
  password?: string;
  realm?: string;
  from_user?: string;
  from_domain?: string;
  transport?: string;
  expire_seconds?: number;
  retry_seconds?: number;
  caller_id_in_from?: boolean;
  extra_params?: Record<string, string>;
  enabled?: boolean;
}

export function listGateways(token: string): Promise<{ gateways: Gateway[] }> {
  return api("/admin/gateways", { token });
}

export function createGateway(token: string, body: GatewayInput): Promise<Gateway> {
  return api("/admin/gateways", { method: "POST", token, body });
}

export function updateGateway(token: string, id: number, body: GatewayInput): Promise<Gateway> {
  return api(`/admin/gateways/${id}`, { method: "PATCH", token, body });
}

export function deleteGateway(token: string, id: number): Promise<void> {
  return api(`/admin/gateways/${id}`, { method: "DELETE", token });
}

export function registerGateway(token: string, id: number): Promise<{ register_status: string }> {
  return api(`/admin/gateways/${id}/register`, { method: "POST", token });
}
