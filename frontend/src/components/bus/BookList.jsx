import {isEmpty} from "lodash-es";
import {PlusOutlined, SettingOutlined} from "@ant-design/icons";
import ContextMenu from "../ContextMenu";
import {Input, message, Statistic} from "antd";
import * as PropTypes from "prop-types";
import {getCacheItem, setCacheItem} from "../Utils";
import classNames from "classnames";
import {DeleteEpubFile, ParseEpubToTxt} from "../../../wailsjs/go/main/App.js";

const TIP = [
    "没有人比我更懂你的需求😀",
    "加油吧打工人💪",
    "打工人，轻松一下😊",
    "Life Is A Fucking Move!😞",
    "什么时候才能不为了一日三餐奔波啊✿",
]

/**
 * book name list
 * @param props
 * @returns {JSX.Element}
 * @constructor
 */
function BookList(props) {

    const state = props.state;

    function sortBookList() {
        let currentBookList = state.currentBookList;

        if (!isEmpty(currentBookList)) {
            let item = getCacheItem("LastClickBook") || "";
            let first = [];
            let last = [];
            (item.split(",")).forEach(e => {
                if (currentBookList.indexOf(e) >= 0) {
                    first.push(e);
                }
            })
            currentBookList.forEach(e => {
                if (first.indexOf(e) < 0) {
                    last.push(e);
                }
            })
            return first.concat(last);
        }

        return currentBookList;
    }


    return <>
        {
            isEmpty(state.currentBookChapterName) && props.display && isEmpty(state.errorInfo) && (
                <div className="book-list">
                    <ul className={"book-list-ul flex flex-column-nowrap"}>
                        <div className="book-list-top-box">
                            <div
                                className={"add-book"}
                                title={"添加文件，右键更多功能"}
                                style={{
                                    "--wails-drop-target": "drop"
                                }} onClick={props.clickBookPlus}>

                                <PlusOutlined/>

                                <ContextMenu
                                    fontSize={16}
                                    options={
                                        [
                                            {label: "去下载"}
                                        ]
                                    }
                                    onSelect={props.onSelect}
                                />
                            </div>
                            <div className={"search-box"}>
                                <Input.Search allowClear={true}
                                              placeholder={"搜索"}
                                              style={{
                                                  width: "100%",
                                                  borderRadius: "none"
                                              }}
                                              onSearch={props.onSearch}
                                />

                                <div className={"flex flex-row gap10 align-item-center"}>
                                    <Statistic title="正在阅读"
                                               value={(getCacheItem("LastClickBook") || "").split(",").filter(Boolean).length + "本"}/>
                                    <Statistic title="拥有图书"
                                               value={state.currentBookList.length + "本"}/>
                                </div>

                            </div>
                        </div>
                        <div className={"flex-auto"}>
                            <div className={"w-100 h-100 over-y-auto"}>
                                {
                                    sortBookList()
                                        .filter(Boolean)
                                        .map((tem, index) => {
                                            return <li
                                                className={classNames("book-list-ul-li", {
                                                    "book-list-ul-li-active": (getCacheItem("LastClickBook") || "").split(",").indexOf(tem) === 0
                                                })}
                                                key={"EpubBook-list-ul-li" + index}
                                                title={tem}
                                                onClick={() => {
                                                    props.clickBookToFirst(tem);
                                                    props.setState({
                                                        loadingBook: true
                                                    })
                                                    if (tem.endsWith("epub")) {

                                                        ParseEpubToTxt(tem).then(res => {
                                                            if (res.startsWith("错误信息:")) {
                                                                message.error(res)
                                                            }

                                                            let s = tem.replace(".epub", ".txt");
                                                            let item = getCacheItem(tem);
                                                            props.reloadBookList(() => {
                                                                props.goChapterByName(s, item, () => {
                                                                    props.setState({
                                                                        loadingBook: false
                                                                    })
                                                                    DeleteEpubFile(tem).then(res => {
                                                                        // hasError(tem)
                                                                        console.log(res)
                                                                    })

                                                                    props.beginRecordTop(tem)
                                                                });
                                                            })

                                                        })
                                                    } else {
                                                        let item = getCacheItem(tem);
                                                        props.goChapterByName(tem, item, () => {
                                                            props.beginRecordTop(tem)

                                                            props.setState({
                                                                loadingBook: false
                                                            })
                                                        });
                                                    }


                                                }}>{tem}</li>
                                        })
                                }
                            </div>

                        </div>

                        <li className={"book-list-ul-li flex align-item-center no-shrink"} style={{
                            width: "100%",
                            justifyContent: "space-between",
                            position: "sticky",
                            bottom: 0,
                            backgroundColor: "#fff",
                            color: "darkred",
                            borderTop: "1px solid rgba(204, 204, 204, 0.32)",
                        }}>
                                    <span>
                                    {
                                        TIP[parseInt(Math.random() * (TIP.length - 1))]
                                    }
                                    </span>
                            <SettingOutlined
                                style={{
                                    fontSize: "20px",
                                    paddingRight: "5px"
                                }}
                                title={"设置！"}
                                onClick={() => {
                                    props.setState({
                                        sysSettingVisible: true
                                    })
                                }}
                            />

                        </li>
                    </ul>

                </div>

            )
        }
    </>;
}

BookList.propTypes = {
    state: PropTypes.any,
    display: PropTypes.bool,
    state1: PropTypes.any,
    clickBookPlus: PropTypes.func,
    onSelect: PropTypes.func,
    onSearch: PropTypes.func,
    clickBookToFirst: PropTypes.func,
    reloadBookList: PropTypes.func,
    goChapterByName: PropTypes.func,
    beginRecordTop: PropTypes.func,
    setState: PropTypes.func,
};

export default BookList