export const CACHE_PREFIX = "Wd-"

export function setCacheItem(item, value) {

    if (item) {
        window.localStorage.setItem(CACHE_PREFIX + item, value);
    }
}

export function getCacheItem(item) {
    if (item) {
        return window.localStorage.getItem(CACHE_PREFIX + item);
    }
}