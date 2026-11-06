import test from 'node:test';
import assert from 'node:assert/strict';
import {
    createUpdateInstaller,
    formatUpdateBytes,
    updateDownloadPercent,
} from '../src/components/updateInstallation.js';

const info = { latestVersion: 'v2.0.6', url: 'https://example.com/releases' };
const deferred = () => {
    let resolve, reject;
    const promise = new Promise((yes, no) => {
        resolve = yes;
        reject = no;
    });
    return { promise, resolve, reject };
};
function harness(install, extra = {}) {
    const listeners = new Map();
    const changes = [];
    const controller = createUpdateInstaller({
        onEvent: (name, callback) => {
            listeners.set(name, callback);
            return () => listeners.delete(name);
        },
        install,
        cancel: async () => {},
        saveProgress: () => {},
        onChange: (state) => changes.push(state),
        ...extra,
    });
    return {
        controller,
        listeners,
        changes,
        emit: (suffix, data) => listeners.get(`wails:updater:${suffix}`)?.({ data }),
        state: () => changes.at(-1),
    };
}

test('fast download events are observed before install resolves and restarting remains busy', async () => {
    let saved = false;
    const h = harness(
        async (version) => {
            assert(saved);
            assert.equal(version, info.latestVersion);
            h.emit('check-started');
            h.emit('download-started', { artifact: { size: 100 } });
            h.emit('download-progress', { written: 42, total: 100, rate: 20 });
            assert.equal(updateDownloadPercent(h.state()), 42);
            h.emit('verifying');
            h.emit('installing');
            h.emit('update-ready');
        },
        {
            saveProgress: () => {
                saved = true;
            },
        },
    );
    await h.controller.run(info);
    assert.deepEqual(
        h.changes.map((state) => state.stage),
        ['preparing', 'checking', 'downloading', 'downloading', 'verifying', 'installing', 'restarting', 'restarting'],
    );
    assert(h.controller.isBusy());
    assert.equal(h.listeners.size, 0);
    const count = h.changes.length;
    await h.controller.run(info);
    assert.equal(h.changes.length, count, 'cannot launch a second update while shutting down');
});

test('network failure retains the diagnostic, removes subscriptions and permits retry', async () => {
    let attempts = 0;
    const h = harness(async () => {
        attempts++;
        h.emit('download-started');
        h.emit('download-progress', { written: 1024, total: 0, rate: 512 });
        assert.equal(updateDownloadPercent(h.state()), null, 'unknown total must not show a fake percentage');
        throw new Error('connection timed out');
    });
    await h.controller.run(info);
    assert.equal(h.state().stage, 'failed');
    assert.match(h.state().error, /connection timed out/);
    assert(!h.controller.isBusy());
    assert.equal(h.listeners.size, 0);
    await h.controller.run(info);
    assert.equal(attempts, 2);
});

test('cancellation waits for backend completion and ignores in-flight progress', async () => {
    const pending = deferred();
    let cancelled = 0;
    const h = harness(() => pending.promise, {
        cancel: async () => {
            cancelled++;
        },
    });
    const running = h.controller.run(info);
    await h.controller.cancel();
    assert.equal(cancelled, 0, 'cannot cancel before the backend session exists');
    h.emit('check-started');
    await h.controller.cancel();
    assert.equal(cancelled, 1);
    assert.equal(h.state().stage, 'cancelling');
    h.emit('download-progress', { written: 50, total: 100 });
    assert.equal(h.state().stage, 'cancelling');
    assert(h.controller.isBusy());
    pending.reject(new Error('context canceled'));
    await running;
    assert.equal(h.state().stage, 'cancelled');
    assert(!h.controller.isBusy());
    assert.equal(h.listeners.size, 0);
});

test('verification and restart cannot be interrupted by the cancel action', async () => {
    const pending = deferred();
    const h = harness(() => pending.promise, {
        cancel: () => {
            assert.fail('cancelled during verification');
        },
    });
    const running = h.controller.run(info);
    h.emit('verifying');
    await h.controller.cancel();
    assert.equal(h.state().stage, 'verifying');
    pending.resolve();
    await running;
});

test('download size formatting handles absent totals and bounds percentages', () => {
    assert.equal(formatUpdateBytes(1024 * 1024 * 12.5), '12.5 MB');
    assert.equal(formatUpdateBytes(undefined), '0 B');
    assert.equal(formatUpdateBytes(-3), '0 B');
    assert.equal(updateDownloadPercent({ written: 110, total: 100 }), 100);
});
