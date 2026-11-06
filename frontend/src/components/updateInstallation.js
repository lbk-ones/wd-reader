const stages = {
    'wails:updater:check-started': 'checking',
    'wails:updater:download-started': 'downloading',
    'wails:updater:download-progress': 'downloading',
    'wails:updater:download-complete': 'verifying',
    'wails:updater:verifying': 'verifying',
    'wails:updater:installing': 'installing',
    'wails:updater:update-ready': 'restarting',
    'reader:update-restarting': 'restarting',
};

const nonNegative = (value) => (Number.isFinite(Number(value)) ? Math.max(0, Number(value)) : 0);

export function formatUpdateBytes(value) {
    const bytes = nonNegative(value);
    const units = ['B', 'KB', 'MB', 'GB'];
    const unit = Math.min(3, Math.floor(Math.log2(Math.max(1, bytes)) / 10));
    return `${(bytes / 1024 ** unit).toFixed(unit ? 1 : 0)} ${units[unit]}`;
}

export function updateDownloadPercent({ written, total }) {
    return total > 0 ? Math.min(100, Math.max(0, Math.round((written / total) * 1000) / 10)) : null;
}

// Register before invoking Go so even a fast check/download cannot outrun the UI.
export function createUpdateInstaller({
    onEvent,
    install,
    cancel,
    saveProgress,
    onChange,
    onError = () => {},
    now = Date.now,
}) {
    let state = null;
    let busy = false;
    let unsubscribe = () => {};
    const publish = (next) => {
        state = next;
        onChange(next);
    };
    return {
        isBusy: () => busy,
        dispose: () => unsubscribe(),
        async run(info) {
            if (busy) return;
            busy = true;
            publish({ info, stage: 'preparing', written: 0, total: 0, rate: 0, lastActivity: now(), error: '' });
            const listeners = [];
            unsubscribe = () => listeners.splice(0).forEach((off) => off());
            try {
                for (const [name, stage] of Object.entries(stages)) {
                    listeners.push(
                        onEvent(name, (event) => {
                            if (!busy || ['cancelling', 'restarting'].includes(state.stage)) return;
                            const data = event.data || {};
                            const progress =
                                name === 'wails:updater:download-progress'
                                    ? {
                                          written: nonNegative(data.written),
                                          total: nonNegative(data.total),
                                          rate: nonNegative(data.rate),
                                      }
                                    : {};
                            if (name === 'wails:updater:download-started')
                                progress.total = nonNegative(data.artifact?.size);
                            const changed =
                                state.stage !== stage || (progress.written ?? state.written) !== state.written;
                            publish({
                                ...state,
                                ...progress,
                                stage,
                                lastActivity: changed ? now() : state.lastActivity,
                            });
                        }),
                    );
                }
                await saveProgress();
                await install(info.latestVersion);
                // Restart returns before the old process has finished shutting down.
                publish({ ...state, stage: 'restarting', lastActivity: now() });
            } catch (error) {
                const cancelled = state.stage === 'cancelling';
                publish({ ...state, stage: cancelled ? 'cancelled' : 'failed', error: cancelled ? '' : String(error) });
                if (!cancelled) onError(error);
                busy = false;
            } finally {
                unsubscribe();
            }
        },
        async cancel() {
            if (!busy || !['checking', 'downloading'].includes(state.stage)) return;
            const previous = state;
            publish({ ...state, stage: 'cancelling' });
            try {
                await cancel();
            } catch (error) {
                if (state.stage === 'cancelling') publish(previous);
                throw error;
            }
        },
    };
}
