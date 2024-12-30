import { useState, useRef, useCallback } from 'react';
import {isFunction} from "ahooks/es/utils";
import {__assign} from "tslib";
import useMemoizedFn from "ahooks/es/useMemoizedFn";
import useUnmountedRef from "ahooks/es/useUnmountedRef";
/**
 * 整合一个关于state的hooks
 * @return
 * state: 对象
 * setState： 合并改变对象
 * getState： 拿到最新的值
 * resetState： 重置对象
 * @author bokun
 * @date 2024-03-11
 */
export default function (initialState){

    let [state,setState] = useState(initialState);

    var unmountedRef = useUnmountedRef();

    let stateRef = useRef(state);
    stateRef.current = state;


    let getState = useCallback(function () {
        return stateRef.current;
    }, []);


    let setMergeState = useCallback(function (patch) {

        // 比较安全的操作
        if (unmountedRef.current) return;

        setState(function (prevState) {
            let newState = isFunction(patch) ? patch(prevState) : patch;
            return newState ? __assign(__assign({}, prevState), newState) : prevState;
        });

    }, []);


    let resetState = useMemoizedFn(function () {
        setState(initialState);
    });

    return [state,setMergeState,getState,resetState]
}
