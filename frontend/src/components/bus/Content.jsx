import {t, useI18n} from '../../i18n/index.jsx';
import {findIndex, isEmpty} from "lodash-es";
import ContextMenu from "../ContextMenu.jsx";
import * as PropTypes from "prop-types";
import {forwardRef, useLayoutEffect} from "react";
import {Tooltip} from "antd";

const Content = (props, ref) => {
    useI18n();
    const state = props.state;
    const settingState = props.settingState;
    const display = props.display;
    let hasDisplay = true;
    if (isEmpty(state.currentBookChapterName) || !display) {
        hasDisplay = false;
    }
    const showBottomBar = state.showTitle && settingState.transparentMode !== "1";
    const bookContainerHeight = props.getBookContainerHeight();
    const chapterIndex = findIndex(state.currentBookChapterList, name => name === state.currentBookChapterName);
    const nextChapterName = state.currentBookChapterList[chapterIndex + 1];
    const previousChapterName = state.currentBookChapterList[chapterIndex - 1];
    const chapterProgress = chapterIndex >= 0 && state.currentBookChapterList.length > 0
        ? (chapterIndex + 1) / state.currentBookChapterList.length * 100 : 0;
    const compactChapter = settingState.miniMode === '1' || settingState.transparentMode === '1';
    useLayoutEffect(() => {
        // A previous chapter may have needed a small alignment gutter at its end.
        if (ref?.current) ref.current.style.paddingBottom = '0px';
    }, [ref, state.currentBookName, state.currentBookChapterName,
        settingState.fontLineHeight, settingState.newContentHeight, compactChapter]);
    return <div

        onContextMenu={props.onContextMenu}
        onClick={props.onClick}
        onDoubleClick={props.onDoubleClick}
        className={"flex flex-auto flex-column-nowrap justify-b"}
        style={{
            backgroundColor: `${settingState.transparentMode !== "1" ? settingState.bgColor : "rgba(0,0,0,0)"}`,
            height: bookContainerHeight + "px", // 这里必须计算一下 很奇怪 flex-auto 不是会自动填充吗？？
            display: hasDisplay ? "flex" : "none"
        }}>
        {
            !state.currentBookName.endsWith(".epub") && (
                <div className={"flex flex-auto shrink-1 w-100"}>
                    <div id={"book-content-id"} ref={ref}
                         onScroll={props.onScroll}
                         onWheel={props.onWheel}
                         onPointerDown={props.onPointerDown}
                         className={"book-content-div flex w-100 flex-column-nowrap no-shrink over-auto"}
                         style={{
                             lineHeight: Number(settingState.fontLineHeight) + "px",
                             '--reader-line-height': Number(settingState.fontLineHeight) + 'px',
                             padding: settingState.miniMode === '1' ? "0 42px 0 10px" : "0 10px",
                             boxSizing: "border-box",
                             textAlign: "left",
                             // textIndent: "2em",
                             height: settingState.newContentHeight > 0 ? settingState.newContentHeight + "px" : '100%',// 必须设置这个高度
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
                                <header className={`reader-chapter${compactChapter ? ' reader-chapter--compact' : ''}`}>
                                    <div className="reader-chapter-index" aria-hidden="true">
                                        {chapterIndex >= 0 ? `${t('章节')} ${String(chapterIndex + 1).padStart(3, '0')}` : t('章节')}
                                    </div>
                                    <h2 className="reader-chapter-title">{state.currentBookChapterName}</h2>
                                </header>
                            )
                        }
                        {
                            state.currentBookChapterContent.map((res, index) => {
                                if (res && res.trim()) {
                                    if (index === 0 && res !== state.currentBookChapterName) {
                                        return <p key={"mulu-li" + index} style={{
                                            textIndent: '2em'
                                        }}>
                                            {res.trim()}
                                        </p>
                                    } else if (index > 0) {
                                        return <p key={"mulu-li" + index} style={{
                                            textIndent: '2em'
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
                                       onClick={props.lastChapter}>{t("上一章")}</a>
                                    <a className={"text-indent0 h-100 cursor-point text-nowrap"}
                                       onClick={props.nextChapter}>{t("下一章")}</a>
                                    <span
                                        className={"book-content-div-footer-btn-txt cursor-point"}
                                        title={state.currentBookChapterList[findIndex(state.currentBookChapterList, e => e === state.currentBookChapterName) + 1] || t("无")}
                                        style={{width: "120px", overflow: "hidden"}}>
                                        {state.currentBookChapterList[findIndex(state.currentBookChapterList, e => e === state.currentBookChapterName) + 1] || t("无")}
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
                <div className={"book-content-div-footer-btn position-relative"}
                     onClick={event => event.stopPropagation()}
                     onDoubleClick={event => event.stopPropagation()}
                     style={{
                    color: settingState.fontColor,
                    zIndex: 30,
                    backgroundColor: `${settingState.transparentMode !== "1" ? settingState.bgColor : "rgba(0,0,0,0)"}`
                }}>
                    <Tooltip title={previousChapterName ? t("上一章：{0}", {0: previousChapterName}) : t("已到全书首章")}>
                        <span className="reader-chapter-nav-slot">
                            <button type="button" className="reader-chapter-nav" disabled={!previousChapterName}
                                    onClick={props.lastChapter}>{t("上一章")}</button>
                        </span>
                    </Tooltip>
                    <div className="reader-chapter-summary">
                        <button type="button" className="reader-current-chapter"
                                title={t("当前章节：{0}（点击打开目录）", {0: state.currentBookChapterName})}
                                aria-label={t("当前章节：{0}，打开目录", {0: state.currentBookChapterName})}
                                onClick={() => props.onSelect({label: t("目录")})}>
                            <span className="reader-chapter-summary-name">{state.currentBookChapterName}</span>
                        </button>
                        <span className="reader-chapter-position">{chapterIndex + 1}/{state.currentBookChapterList.length}</span>
                    </div>
                    <Tooltip title={nextChapterName ? t("下一章：{0}", {0: nextChapterName}) : t("已到全书末章")}>
                        <span className="reader-chapter-nav-slot">
                            <button type="button" className="reader-chapter-nav" disabled={!nextChapterName}
                                    onClick={props.nextChapter}>{t("下一章")}</button>
                        </span>
                    </Tooltip>
                    {settingState.showProgress === '1' && (
                        <div className="reader-progress" role="progressbar"
                             aria-label={t("全书章节进度")} aria-valuemin={0} aria-valuemax={100}
                             aria-valuenow={Math.round(chapterProgress * 10) / 10}
                             aria-valuetext={t("第 {0} 章，共 {1} 章", {0: chapterIndex + 1, 1: state.currentBookChapterList.length})}
                             title={t("章节进度 {0}/{1} · {2}%", {0: chapterIndex + 1, 1: state.currentBookChapterList.length, 2: Math.round(chapterProgress)})}>
                            <div className="reader-progress-track" aria-hidden="true"/>
                            <div className="reader-progress-fill" aria-hidden="true" style={{width: `${chapterProgress}%`}}/>
                        </div>
                    )}


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
