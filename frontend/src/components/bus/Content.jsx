import {findIndex, isEmpty} from "lodash-es";
import ContextMenu from "../ContextMenu.jsx";
import * as PropTypes from "prop-types";
import {forwardRef, useEffect} from "react";
import {Progress} from "antd";

const Content = (props,ref)=>{
    const state = props.state;
    const settingState = props.settingState;
    const display = props.display;
    if (isEmpty(state.currentBookChapterName) || !display){
        return null
    }
    const showBottomBar = state.showTitle && settingState.transparentMode !== "1";
    const bookContainerHeight = props.getBookContainerHeight();
    return <div

            onContextMenu={props.onContextMenu}
            onClick={props.onClick}
            onDoubleClick={props.onDoubleClick}
            className={"flex flex-auto flex-column-nowrap justify-b"}
            style={{
                backgroundColor:`${settingState.transparentMode !== "1" ? settingState.bgColor : "rgba(0,0,0,0)"}`,
                height:bookContainerHeight+"px" // 这里必须计算一下 很奇怪 flex-auto 不是会自动填充吗？？
            }}>
        {
            !state.currentBookName.endsWith(".epub") && (
                <div className={"flex flex-auto shrink-1 w-100 h-100"}>
                    <div id={"book-content-id"} ref={ref} className={"book-content-div flex w-100 flex-column-nowrap no-shrink over-auto"}
                         style={{
                             lineHeight: Number(settingState.fontLineHeight) + "px",
                             padding: "0 10px",
                             boxSizing: "border-box",
                             textAlign: "left",
                             // textIndent: "2em",
                             height: settingState.newContentHeight>0?settingState.newContentHeight+"px":'100%',// 必须设置这个高度
                             // marginBottom: settingState.blankLineHeight + "px",
                             color: settingState.fontColor,
                             fontSize: Number(settingState.fontSize) + "px",
                             visibility: !display ? "hidden" : "visible",
                             backgroundColor: `${settingState.transparentMode !== "1" ? settingState.bgColor : "rgba(0,0,0,0)"}`
                         }}

                    >
                        {
                            !isEmpty(state.currentBookChapterName)
                            && (
                                <p className={"font-bold"} style={{
                                    textIndent:'2em'
                                }}>
                                    {state.currentBookChapterName || ""}
                                </p>
                            )
                        }
                        {
                            state.currentBookChapterContent.map((res, index) => {
                                if (res && res.trim()) {
                                    if (index === 0 && res !== state.currentBookChapterName) {
                                        return <p key={"mulu-li" + index} style={{
                                            textIndent:'2em'
                                        }}>
                                            {res.trim()}
                                        </p>
                                    } else if (index > 0) {
                                        return <p key={"mulu-li" + index} style={{
                                            textIndent:'2em'
                                        }}>
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
                                        title={state.currentBookChapterList[findIndex(state.currentBookChapterList, e => e === state.currentBookChapterName) + 1] || "无"}
                                        style={{width: "120px", overflow: "hidden"}}>
                                        {state.currentBookChapterList[findIndex(state.currentBookChapterList, e => e === state.currentBookChapterName) + 1] || "无"}
                                    </span>
                                </div>
                            )
                        }


                        <ContextMenu options={props.options} onSelect={props.onSelect}/>
                    </div>
                </div>

            )
        }
        {/*非透明模式下的下一章*/}
        {
            showBottomBar && (
                <div className={"book-content-div-footer-btn position-relative"} style={{
                    color: settingState.fontColor,
                    zIndex:30,
                    backgroundColor: `${settingState.transparentMode !== "1" ? settingState.bgColor : "rgba(0,0,0,0)"}`
                }}>
                    <a className={"text-indent0 h-100 cursor-point no-shrink text-nowrap"} style={{
                        flexBasis:120,
                    }} onClick={props.lastChapter}>上一章</a>
                    <a className={"text-indent0 h-100 cursor-point no-shrink text-nowrap"} style={{
                        flexBasis:120,
                    }} onClick={props.nextChapter}>下一章</a>
                    <span
                        className={"book-content-div-footer-btn-txt cursor-point flex-auto text-over-dot"}
                        title={state.currentBookChapterList[findIndex(state.currentBookChapterList, e => e === state.currentBookChapterName) + 1] || "无"}
                    >
                        {state.currentBookChapterList[findIndex(state.currentBookChapterList, e => e === state.currentBookChapterName) + 1] || "无"}
                    </span>
                    {/*添加个进度条*/}
                    {
                        settingState.showProgress === '1' && (

                        <div className={"position-absolute w-100"} style={{
                            height:5,
                            left:0,
                            bottom:0,
                            zIndex:20,
                            lineHeight:0
                        }}>
                            <Progress
                                style={{
                                    width:'100%',
                                    height:'100%',
                                    lineHeight:0
                                }}
                                type={"line"}
                                showInfo={false}
                                percent={(function (){
                                    let currentBookChapterList = state.currentBookChapterList;
                                    let currentBookChapterName = state.currentBookChapterName;
                                    let index = findIndex(currentBookChapterList, e => e === currentBookChapterName);
                                    let length = currentBookChapterList.length;
                                    if(length!==0){
                                        let number = index/length;
                                        number = (number === 0 || number === Infinity) ? 0.01 : number
                                        return Math.ceil(number*100)
                                    }else{
                                        return 1
                                    }
                                })()}
                                percentPosition={{ align: 'start', type: 'inner' }}
                                size={["100%", 5]}
                                strokeColor="#f60"
                            />
                        </div>
                        )
                    }


                </div>
            )
        }
    </div>;
}


// Content.propTypes = {
//     state: PropTypes.any,
//     settingState: PropTypes.any,
//     display: PropTypes.bool,
//     ref: PropTypes.any,
//     onContextMenu: PropTypes.func,
//     getBookContainerHeight: PropTypes.func,
//     onClick: PropTypes.func,
//     onDoubleClick: PropTypes.func,
//     lastChapter: PropTypes.func,
//     nextChapter: PropTypes.func,
//     options: PropTypes.any,
//     onSelect: PropTypes.func
// };
export default forwardRef(Content)