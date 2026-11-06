const own = (value, key) => Object.prototype.hasOwnProperty.call(value, key);
const record = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);

function readStorage(storage) {
    const result = Object.create(null);
    for (let index = 0; index < storage.length; index++) {
        const key = storage.key(index);
        if (key !== null) result[key] = storage.getItem(key);
    }
    return result;
}

export function snapshotFrontendCache(storages = window) {
    return {
        localStorage: readStorage(storages.localStorage),
        sessionStorage: readStorage(storages.sessionStorage),
    };
}

function splitMap(value) {
    if (!value) return Object.create(null);
    const result = JSON.parse(value);
    if (!record(result)) throw new Error('Invalid chapter settings');
    return result;
}

export function mergeCacheEntries(current, incoming, bookNames = {}) {
    if (!record(incoming)) throw new Error('Invalid frontend cache');
    const result = Object.assign(Object.create(null), current);
    const renameBook = (name) => (own(bookNames, name) ? bookNames[name] : name);
    for (const [key, value] of Object.entries(incoming)) {
        if (typeof value !== 'string') throw new Error('Invalid frontend cache value');
        if (key === 'Wd-LastClickBook') {
            const history = [...(current[key] || '').split(','), ...value.split(',').map(renameBook)];
            result[key] = [...new Set(history.filter(Boolean))].join(',');
        } else if (key === 'Wd-SplitMap') {
            const merged = Object.assign(Object.create(null), splitMap(current[key]));
            for (const [name, setting] of Object.entries(splitMap(value))) {
                const mapped = own(bookNames, name)
                    ? renameBook(name)
                    : name.endsWith('-Value')
                      ? renameBook(name.slice(0, -6)) + '-Value'
                      : name;
                if (!own(merged, mapped)) merged[mapped] = setting;
            }
            result[key] = JSON.stringify(merged);
        } else {
            let mapped = key;
            if (key.startsWith('Wd-scroll-top')) mapped = 'Wd-scroll-top' + renameBook(key.slice(13));
            else if (key.startsWith('Wd-')) mapped = 'Wd-' + renameBook(key.slice(3));
            if (!own(result, mapped)) result[mapped] = value;
        }
    }
    return result;
}

// Apply both storage areas as one operation, with rollback if quota or the disk commit fails.
export function mergeFrontendCache(cache, bookNames = {}, storages = window) {
    const before = snapshotFrontendCache(storages);
    const plans = ['localStorage', 'sessionStorage'].map((area) => ({
        area,
        values: mergeCacheEntries(before[area], cache[area] ?? {}, bookNames),
    }));
    const changes = [];
    const rollback = () => {
        let failure;
        for (const { area, key, previous } of changes.slice().reverse()) {
            try {
                if (previous === null) storages[area].removeItem(key);
                else storages[area].setItem(key, previous);
            } catch (error) {
                failure = error;
            }
        }
        if (failure) throw failure;
    };
    try {
        for (const { area, values } of plans) {
            for (const [key, value] of Object.entries(values)) {
                const previous = storages[area].getItem(key);
                if (previous === value) continue;
                storages[area].setItem(key, value);
                changes.push({ area, key, previous });
            }
        }
    } catch (error) {
        rollback();
        throw error;
    }
    return { count: changes.length, rollback };
}
