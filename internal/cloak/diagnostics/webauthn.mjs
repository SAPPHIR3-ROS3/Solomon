import { detached, rejectPaths } from './shared.mjs';

const tabs = new Map();
const structural = ['protocol', 'transport', 'hasResidentKey', 'hasUserVerification'];
const protocols = ['ctap2', 'u2f'];
const transports = ['internal', 'usb', 'nfc', 'ble', 'hybrid'];

export async function cleanup(tabID) {
  const key = String(tabID);
  const record = tabs.get(key);
  if (!record) return;
  tabs.delete(key);
  record.page.off('close', record.onClose);
  if (record.id) await record.session.send('WebAuthn.removeVirtualAuthenticator', { authenticatorId: record.id }).catch(() => undefined);
  await record.session.send('WebAuthn.disable').catch(() => undefined);
  await detached(record.session);
}

export async function dispatch(name, args = {}, state) {
  if (name !== 'configure_webauthn') return undefined;
  rejectPaths(args);
  if (typeof args.enabled !== 'boolean') throw new Error('enabled must be boolean');
  const key = String(state.tabID);
  if (!args.enabled) { await cleanup(key); return { enabled: false, scope: 'Authenticators owned by this module only' }; }
  if (args.protocol !== undefined && !protocols.includes(args.protocol)) throw new Error('Unsupported WebAuthn protocol');
  if (args.transport !== undefined && !transports.includes(args.transport)) throw new Error('Unsupported WebAuthn transport');
  for (const field of ['hasResidentKey', 'hasUserVerification', 'isUserVerified']) {
    if (args[field] !== undefined && typeof args[field] !== 'boolean') throw new Error(field + ' must be boolean');
  }
  let record = tabs.get(key);
  const options = { protocol: 'ctap2', transport: 'internal', hasResidentKey: true, hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true, ...record?.options };
  for (const field of [...structural, 'isUserVerified']) if (args[field] !== undefined) options[field] = args[field];
  if (options.protocol === 'u2f') { options.hasResidentKey = false; options.hasUserVerification = false; }
  if (!record) {
    const session = await state.context.newCDPSession(state.page);
    try { await session.send('WebAuthn.enable', { enableUI: false }); } catch (error) { await detached(session); throw error; }
    record = { session, page: state.page };
    record.onClose = () => { void cleanup(key); };
    state.page.on('close', record.onClose);
    tabs.set(key, record);
  }
  const replaced = Boolean(record.id) && structural.some(field => record.options[field] !== options[field]);
  try {
    if (replaced) {
      await record.session.send('WebAuthn.removeVirtualAuthenticator', { authenticatorId: record.id });
      record.id = undefined;
    }
    if (!record.id) {
      const created = await record.session.send('WebAuthn.addVirtualAuthenticator', { options });
      record.id = created.authenticatorId;
    } else if (record.options.isUserVerified !== options.isUserVerified) {
      await record.session.send('WebAuthn.setUserVerified', { authenticatorId: record.id, isUserVerified: options.isUserVerified });
    }
    record.options = options;
    return { enabled: true, authenticatorId: record.id, options, replaced, credentialsDeleted: replaced };
  } catch (error) {
    await cleanup(key);
    throw error;
  }
}
