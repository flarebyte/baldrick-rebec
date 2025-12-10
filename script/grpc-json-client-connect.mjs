// Minimal Connect JSON client using generated ES descriptors in script/gen.
//
// - Uses ES output at script/gen/prompt/v1/prompt_pb.js for validation/coercion
// - Sends HTTP POST application/connect+json to the Connect endpoint
// - Returns plain JSON normalized via response schema
//

import { fromJson, toJson } from '@bufbuild/protobuf';
import * as promptpb from './gen/prompt/v1/prompt_pb.js';

// Optional: interceptor to add/override headers (e.g., force JSON content-type for experiments)
// No interceptors; simple fetch-based client

// Creates a Connect client and exposes RPCs that accept/return plain JSON.
export function createConnectGrpcJsonClient({ baseUrl, headers = {} }) {
  if (!baseUrl) throw new Error('baseUrl is required');
  const endpoint = `${baseUrl.replace(/\/$/, '')}/prompt.v1.PromptService/Run`;
  return {
    async Run(jsonReq = {}) {
      // Validate request via schema
      try {
        fromJson(promptpb.PromptRunRequestSchema, jsonReq);
      } catch (e) {
        const detail = e?.message ?? String(e);
        throw new Error(`request validation failed: ${detail}`);
      }
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'content-type': 'application/connect+json', ...headers },
        body: JSON.stringify(jsonReq),
      });
      const text = await res.text();
      let obj;
      try {
        obj = JSON.parse(text || 'null');
      } catch {
        throw new Error(`invalid JSON response: ${text.slice(0, 200)}`);
      }
      if (obj && obj.error) {
        const code = obj.error?.code || 'unknown';
        const msg = obj.error?.message || 'error';
        throw new Error(`connect error ${code}: ${msg}`);
      }
      // Validate response via schema; normalize to JSON mapping
      try {
        const msg = fromJson(promptpb.PromptRunResponseSchema, obj);
        return toJson(promptpb.PromptRunResponseSchema, msg, {
          emitDefaultValues: false,
        });
      } catch (e) {
        const detail = e?.message ?? String(e);
        throw new Error(`response validation failed: ${detail}`);
      }
    },
  };
}
