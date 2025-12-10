// Minimal gRPC JSON client for Node using HTTP/2 and protobufjs reflection.
// - Loads a .proto file to discover service/methods and message types
// - Validates requests/responses via protobufjs Type.verify/fromObject
// - Sends application/grpc+json requests framed per gRPC over HTTP/2

import http2 from 'node:http2';
import protobufjs from 'protobufjs';

const CONTENT_TYPE = 'application/grpc+json';

function frameMessage(jsonBytes) {
  const body = Buffer.from(jsonBytes);
  const header = Buffer.allocUnsafe(5);
  header.writeUInt8(0, 0); // uncompressed
  header.writeUInt32BE(body.length, 1);
  return Buffer.concat([header, body]);
}

function parseUnaryResponse(chunks) {
  const buf = Buffer.concat(chunks);
  if (buf.length < 5) {
    // Fallback: some servers may return plain JSON without gRPC framing
    try {
      return JSON.parse(buf.toString('utf8') || 'null');
    } catch {
      throw new Error('grpc: short response');
    }
  }
  const flag = buf.readUInt8(0);
  const len = buf.readUInt32BE(1);
  if (flag !== 0) throw new Error('grpc: compression not supported');
  if (buf.length < 5 + len) throw new Error('grpc: incomplete message');
  const msg = buf.subarray(5, 5 + len).toString('utf8');
  return JSON.parse(msg || 'null');
}

export async function createGrpcJsonClient({
  baseUrl,
  protoPath,
  serviceName,
}) {
  if (!baseUrl || !protoPath || !serviceName) {
    throw new Error('baseUrl, protoPath, serviceName are required');
  }
  const pb =
    protobufjs && typeof protobufjs.load === 'function'
      ? protobufjs
      : protobufjs?.default && typeof protobufjs.default.load === 'function'
        ? protobufjs.default
        : null;
  if (!pb) {
    throw new Error(
      'protobuf.load is not available; ensure protobufjs is installed correctly',
    );
  }
  const root = await pb.load(protoPath);
  const svc = root.lookupService(serviceName);
  if (!svc) throw new Error(`service not found: ${serviceName}`);

  function openSession() {
    const u = new URL(baseUrl);
    const authority = `${u.hostname}${u.port ? `:${u.port}` : ''}`;
    const session = http2.connect(`${u.protocol}//${authority}`);
    session.on('error', () => {
      /* surface per-call */
    });
    return session;
  }

  const client = {};
  for (const m of svc.methodsArray) {
    const reqType = root.lookupType(m.requestType);
    const resType = root.lookupType(m.responseType);
    const path = `/${svc.fullName.replace(/^\./, '')}/${m.name}`;
    client[m.name] = (reqObj, { timeoutMs = 30000 } = {}) =>
      new Promise((resolve, reject) => {
        // Validate/coerce request.
        // protobufjs reflection can be strict for google.protobuf.Value (input). We validate other fields
        // and allow native JSON for input by skipping its strict validation when necessary.
        let jsonPayload;
        try {
          const msg = reqType.fromObject(reqObj || {});
          jsonPayload = reqType.toObject(msg, { json: true });
        } catch (e1) {
          // Verify without the 'input' field; if any remaining problems, surface them.
          const shallow = { ...(reqObj || {}) };
          delete shallow.input;
          const vmsg = reqType.verify(shallow);
          if (vmsg && String(vmsg).trim() !== '') {
            return reject(
              new Error(
                `request validation failed: ${vmsg}; cause: ${
                  e1?.message || e1
                }`,
              ),
            );
          }
          jsonPayload = reqObj || {};
        }
        // For JSON codec, send JSON mapping of the proto message
        const jsonBytes = Buffer.from(JSON.stringify(jsonPayload));
        const framed = frameMessage(jsonBytes);

        const session = openSession();
        const headers = {
          ':method': 'POST',
          ':path': path,
          'content-type': CONTENT_TYPE,
          te: 'trailers',
          'grpc-accept-encoding': 'identity',
          'grpc-encoding': 'identity',
        };
        const req = session.request(headers, { endStream: false });
        const chunks = [];
        let trailers = {};
        let finished = false;

        const done = (err, val) => {
          if (finished) return;
          finished = true;
          try {
            session.close();
          } catch {}
          if (err) return reject(err);
          return resolve(val);
        };

        req.on('response', () => {});
        req.on('trailers', (t) => {
          trailers = t;
        });
        req.on('data', (c) => chunks.push(Buffer.from(c)));
        req.on('error', (e) => done(e));
        req.on('end', () => {
          const status = Number(trailers['grpc-status'] || 0);
          const message = String(trailers['grpc-message'] || '');
          if (status !== 0)
            return done(new Error(`grpc error ${status}: ${message}`));
          try {
            const obj = parseUnaryResponse(chunks);
            // Coerce + validate response
            const msg = resType.fromObject(obj || {});
            const normalized = resType.toObject(msg, { json: true });
            return done(null, normalized);
          } catch (e) {
            return done(
              new Error(`response validation failed: ${e?.message || e}`),
            );
          }
        });

        // Send body then end stream
        req.end(framed);

        if (timeoutMs > 0) {
          const t = setTimeout(
            () => done(new Error('grpc timeout')),
            timeoutMs,
          );
          req.on('close', () => clearTimeout(t));
        }
      });
  }

  return client;
}
