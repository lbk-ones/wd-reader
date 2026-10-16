export const CACHE_PREFIX = "Wd-"
const CACHE_CHANGE_EVENT = 'wd-cache-change';

export function getCacheCount() {
    return Object.keys(window.localStorage).filter(key => key.startsWith(CACHE_PREFIX)).length;
}

export function subscribeCacheChanges(listener) {
    const onStorage = event => {
        if (event.storageArea === window.localStorage && (event.key === null || event.key.startsWith(CACHE_PREFIX))) {
            listener();
        }
    };
    window.addEventListener(CACHE_CHANGE_EVENT, listener);
    window.addEventListener('storage', onStorage);
    return () => {
        window.removeEventListener(CACHE_CHANGE_EVENT, listener);
        window.removeEventListener('storage', onStorage);
    };
}

export function clearCache() {
    const keys = Object.keys(window.localStorage).filter(key => key.startsWith(CACHE_PREFIX));
    keys.forEach(key => window.localStorage.removeItem(key));
    window.dispatchEvent(new Event(CACHE_CHANGE_EVENT));
}

export function clearReadingHistory() {
    const keys = Object.keys(window.localStorage).filter(key => key.startsWith(CACHE_PREFIX)
        && (key === CACHE_PREFIX + 'LastClickBook'
            || key.startsWith(CACHE_PREFIX + 'scroll-top')
            || /\.(txt|epub|mobi|azw|azw3)$/i.test(key)));
    keys.forEach(key => window.localStorage.removeItem(key));
    window.dispatchEvent(new Event(CACHE_CHANGE_EVENT));
}

export function setCacheItem(item, value) {

    if (item) {
        window.localStorage.setItem(CACHE_PREFIX + item, value);
        window.dispatchEvent(new Event(CACHE_CHANGE_EVENT));
    }
}

export function getCacheItem(item) {
    if (item) {
        return window.localStorage.getItem(CACHE_PREFIX + item);
    }
}

const SPLIT_TYPE = "SplitMap"

export function putSplitMap(key,value){
    const cacheItem = getCacheItem(SPLIT_TYPE);
    if(cacheItem){
        let parse = JSON.parse(cacheItem);
        parse[key]=value
        setCacheItem(SPLIT_TYPE,JSON.stringify(parse))
    }else{
        let te = {}
        te[key]=value
        setCacheItem(SPLIT_TYPE,JSON.stringify(te));
    }
    return getSplitMap()
}

export function deleteSplitMapByKey(key){
    let splitMap = getSplitMap();

    splitMap[key] = "";
    putSplitMap(key,JSON.stringify(splitMap))
}

export function getSplitMap(){
    const cacheItem = getCacheItem(SPLIT_TYPE);
    try{
        if(cacheItem){
            return JSON.parse(cacheItem);
        }
    }catch (e){

    }
    return {}
}
export function getSplitType(currentBookName){
    if(currentBookName){
        let splitMap = getSplitMap();
        return (splitMap[currentBookName] || '1')+""
    }
    return "1";


}
export function getSplitTypeValue(currentBookName){
    if(currentBookName){
        let splitMap = getSplitMap();
        return (splitMap[currentBookName+"-Value"] || "")+""
    }
    return ""
}
export function setSplitTypeValue(currentBookName,value){
    return putSplitMap(currentBookName+"-Value",value)

}
