// Connect-Node based client using generated ES descriptors in script/gen.
//
// - Uses @bufbuild/protoc-gen-es output at script/gen/prompt/v1/prompt_pb.js
// - Builds a Connect ServiceType at runtime (no connect-es required)
// - Validates inputs via Schema.fromJson and returns outputs via Schema.toJson
//
import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-node';
import * as promptpb from './gen/prompt/v1/prompt_pb.js';

// Optional: interceptor to add/override headers (e.g., force JSON content-type for experiments)
function headerInterceptor(extraHeaders = {}) {
  return (next) => async (req) => {
    // Merge headers (case-insensitive keys by convention)
    for (const [k, v] of Object.entries(extraHeaders)) {
      req.header.set(k, String(v));
    }
    return await next(req);
  };
}

// Creates a Connect client and exposes RPCs that accept/return plain JSON.
export function createConnectGrpcJsonClient({ baseUrl, headers = {} }) {
  if (!baseUrl) throw new Error('baseUrl is required');
  // Build a Connect ServiceType from ES descriptors
  const ServiceType = {
    typeName: 'prompt.v1.PromptService',
    methods: [
      {
        name: 'Run',
        kind: 'unary',
        I: promptpb.PromptRunRequestSchema,
        O: promptpb.PromptRunResponseSchema,
      },
    ],
  };

  const transport = createConnectTransport({
    baseUrl,
    interceptors: Object.keys(headers).length ? [headerInterceptor(headers)] : undefined,
  });
  const c = createPromiseClient(ServiceType, transport);

  return {
    async Run(jsonReq = {}, options = {}) {
      // Validate/coerce input JSON into a Message via fromJson
      let msgIn;
      try {
        msgIn = promptpb.PromptRunRequestSchema.fromJson(jsonReq, { ignoreUnknownFields: false });
      } catch (e) {
        const detail = e?.message || String(e);
        throw new Error(`request validation failed: ${detail}`);
      }
      try {
        const res = await c.Run(msgIn, options);
        return promptpb.PromptRunResponseSchema.toJson(res, { emitDefaultValues: false });
      } catch (e) {
        const code = e?.code !== undefined ? ` code=${e.code}` : '';
        const msg = e?.message || String(e);
        throw new Error(`rpc Run failed:${code} ${msg}`);
      }
    },
  };
}
