// Minimal gRPC JSON client for Node using HTTP/2 and protobufjs reflection.
// - Loads a .proto file to discover service/methods and message types
// - Validates requests/responses via protobufjs Type.verify/fromObject
// - Sends application/grpc+json requests framed per gRPC over HTTP/2

import http2 from 'node:http2';
import * as protobuf from 'protobufjs';

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
  if (buf.length < 5) throw new Error('grpc: short response');
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
  const root = await protobuf.load(protoPath);
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
        // Validate request
        const errMsg = reqType.verify(reqObj);
        if (errMsg)
          return reject(new Error(`request validation failed: ${errMsg}`));
        // For JSON codec, we send plain JSON matching the message shape
        const jsonBytes = Buffer.from(JSON.stringify(reqObj));
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
            // Validate response
            const errResp = resType.verify(obj);
            if (errResp)
              return done(new Error(`response validation failed: ${errResp}`));
            return done(null, obj);
          } catch (e) {
            return done(e);
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
