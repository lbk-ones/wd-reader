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