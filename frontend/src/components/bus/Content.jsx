import {findIndex, isEmpty} from "lodash-es";
import ContextMenu from "../ContextMenu.jsx";
import * as PropTypes from "prop-types";
import {forwardRef} from "react";

const Content = (props,ref)=>{
    const state = props.state;
    const settingState = props.settingState;
    const display = props.display;
    return <>
        {
            !isEmpty(state.currentBookChapterName) && !state.currentBookName.endsWith(".epub") && (
                <div className={"book-content-div"}
                     style={{
                         lineHeight: Number(settingState.fontLineHeight) + "px",
                         padding: "0 10px",
                         boxSizing: "border-box",
                         textAlign: "left",
                         textIndent: "2em",
                         display: "flex",
                         flexFlow: "column nowrap",
                         flex: "auto",
                         overflow: "auto",
                         color: settingState.fontColor,
                         fontSize: Number(settingState.fontSize) + "px",
                         visibility: !display ? "hidden" : "visible",
                         backgroundColor: `${settingState.transparentMode !== "1" ? settingState.bgColor : "rgba(0,0,0,0)"}`
                     }}
                     ref={ref}
                     onContextMenu={props.onContextMenu}
                     onClick={props.onClick}
                     onDoubleClick={props.onDoubleClick}
                >
                    <div className={"flex flex-auto flex-column"}>
                        <div className={"w-100 h-100"}>
                            {
                                !isEmpty(state.currentBookChapterName)
                                && (
                                    <p className={"mb-5 font-bold"}>
                                        {state.currentBookChapterName || ""}
                                    </p>
                                )
                            }
                            {
                                state.currentBookChapterContent.map((res,index)=>{
                                    if (res && res.trim()) {
                                        if (index === 0 && res !== state.currentBookChapterName) {
                                            return <p key={"mulu-li" + index}>
                                                {res.trim()}
                                            </p>
                                        } else if (index > 0) {
                                            return <p key={"mulu-li" + index}>
                                                {res.trim()}
                                            </p>
                                        }
                                    }
                                    return null;
                                })
                            }

                            {/*透明模式下的到底部分页*/}
                            {
                                settingState.transparentMode === "1" && (
                                    <div className={"flex flex-row-nowrap justify-b"} style={{
                                        color: settingState.fontColor,
                                        backgroundColor: `${settingState.transparentMode !== "1" ? settingState.bgColor : "rgba(0,0,0,0)"}`
                                    }}>
                                        <a className={"text-indent0 h-100 cursor-point text-nowrap"}
                                           onClick={props.lastChapter}>上一章</a>
                                        <a className={"text-indent0 h-100 cursor-point text-nowrap"}
                                           onClick={props.nextChapter}>下一章</a>
                                        <span
                                            className={"book-content-div-footer-btn-txt cursor-point"}
                                            title={state.currentBookChapterList[findIndex(state.currentBookChapterList, props.predicate) + 1] || "无"}
                                            style={{width: "120px", overflow: "hidden"}}>
                                        {state.currentBookChapterList[findIndex(state.currentBookChapterList, e => e === state.currentBookChapterName) + 1] || "无"}
                                    </span>
                                    </div>
                                )
                            }

                        </div>
                    </div>

                    {/*非透明模式下的下一章*/}
                    {
                        state.showTitle && settingState.transparentMode !== "1" && (
                            <div className={"book-content-div-footer-btn"} style={{
                                color: settingState.fontColor,
                                backgroundColor: `${settingState.transparentMode !== "1" ? settingState.bgColor : "rgba(0,0,0,0)"}`
                            }}>
                                <a className={"text-indent0 h-100 cursor-point text-nowrap"} onClick={props.lastChapter}>上一章</a>
                                <a className={"text-indent0 h-100 cursor-point text-nowrap"} onClick={props.nextChapter}>下一章</a>
                                <span
                                    className={"book-content-div-footer-btn-txt cursor-point"}
                                    title={state.currentBookChapterList[findIndex(state.currentBookChapterList, props.predicate) + 1] || "无"}
                                    style={{width: "120px", overflow: "hidden"}}
                                >
                                        {state.currentBookChapterList[findIndex(state.currentBookChapterList, props.predicate) + 1] || "无"}
                                </span>
                            </div>
                        )
                    }


                    <ContextMenu options={props.options} onSelect={props.onSelect}/>
                </div>
            )
        }
    </>;
}


Content.propTypes = {
    state: PropTypes.any,
    settingState: PropTypes.any,
    display: PropTypes.bool,
    ref: PropTypes.any,
    onContextMenu: PropTypes.func,
    onClick: PropTypes.func,
    onDoubleClick: PropTypes.func,
    lastChapter: PropTypes.func,
    nextChapter: PropTypes.func,
    options: PropTypes.any,
    onSelect: PropTypes.func
};
export default forwardRef(Content)