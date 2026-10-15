import {isEmpty} from "lodash-es";
import {
    DeleteOutlined,
    BookOutlined,
    ReadOutlined,
    DownloadOutlined,
    GithubOutlined,
    PlusOutlined,
    SettingOutlined
} from "@ant-design/icons";
import {Button, Input, message} from "antd";
import {getCacheItem} from "../Utils";
import classNames from "classnames";
import {DeleteEpubFile, DeleteFile, GetScOne, ParseEpubToTxt} from "../../../bindings/wd-reader/app";
import {forwardRef, useEffect, useImperativeHandle, useState} from "react";
import {Browser} from "@wailsio/runtime";

const TIP = ["翻开一页，暂别喧嚣。", "每一段闲暇，都有书相伴。", "慢慢读，世界会安静下来。"]

/**
 * book name list
 * @param props
 * @returns {JSX.Element}
 * @constructor
 */
function BookList(props, ref) {

    const state = props.state;

    const [sc, setSc] = useState("")

    useEffect(function () {
        GetScOne().then(res => {
            if (res) {
                try {
                    let parse = JSON.parse(res);
                    setSc(parse.hitokoto || TIP[parseInt(Math.random() * (TIP.length - 1))])
                } catch (e) {

                }
            } else {
                setSc(res || TIP[parseInt(Math.random() * (TIP.length - 1))])
            }
        })
    }, [])

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

    function selectBookFun(tem) {
        props.clickBookToFirst(tem);
        props.setState({
            loadingBook: true
        })
        if (tem.endsWith("epub")) {

            ParseEpubToTxt(tem).then(res => {
                if (res.startsWith("错误信息:")) {
                    message.error(res)
                    return;
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

                        props.calculateFontLines()
                    });
                })

            })
        } else {
            let item = getCacheItem(tem);
            console.log('item', item)
            console.log('tem', tem)
            props.goChapterByName(tem, item, () => {
                props.beginRecordTop(tem)

                props.setState({
                    loadingBook: false

                })
                props.calculateFontLines()
            });
        }
    }

    useImperativeHandle(ref, () => ({
        selectBook: (tem) => {
            if (tem) {
                selectBookFun(tem)
            }
        }
    }))

    const books = sortBookList().filter(Boolean);
    const recentBooks = (getCacheItem("LastClickBook") || "").split(",").filter(Boolean);

    return isEmpty(state.currentBookChapterName) && props.display && isEmpty(state.errorInfo) ? (
        <section className="book-list" aria-label="我的书架">
            <div className="library-heading">
                <div className="library-eyebrow">YOUR READING SPACE</div>
                <div className="library-heading-row">
                    <h1>我的书架<span className="library-count">{books.length}</span></h1>
                    <span className="library-version">v{state.version || 'dev'}</span>
                </div>
                <p>把片刻留给阅读。</p>
                <div className="library-actions">
                    <Button type="primary" icon={<PlusOutlined/>} onClick={props.clickBookPlus}>
                        添加书籍
                    </Button>
                    <Button icon={<DownloadOutlined/>} onClick={() => props.onSelect({label: "去下载"})}>
                        图书下载
                    </Button>
                </div>
                <Input.Search className="library-search" allowClear placeholder="搜索书名或拼音首字母"
                              aria-label="搜索书籍" onSearch={props.onSearch}/>
            </div>
            <div className="library-section-label">
                <span><ReadOutlined/> 藏书一览</span>
                <span>{recentBooks.length ? recentBooks.length + ' 本阅读足迹' : '双击书籍，开始阅读'}</span>
            </div>
            <ul className="book-list-ul" aria-label="书籍列表">
                {books.map((name, index) => {
                    const title = name.replace(/\.(txt|epub)$/i, '');
                    const isRecent = recentBooks[0] === name;
                    return <li key={name} className={classNames('library-book', {'library-book--recent': isRecent})}>
                        <button className="library-book-open" title={'双击阅读：' + title}
                                onDoubleClick={() => selectBookFun(name)}
                                onKeyDown={event => {if (event.key === 'Enter') selectBookFun(name)}}>
                            <span className={'book-spine book-spine--' + index % 4} aria-hidden="true">
                                <span>{title.replace(/[《【「]/g, '').slice(0, 1)}</span>
                                <BookOutlined/>
                            </span>
                            <span className="library-book-info">
                                <span className="library-book-title">{title}</span>
                                <span className="library-book-meta">
                                    <span>{name.split('.').pop().toUpperCase()}</span>
                                    <span className="meta-dot">·</span>
                                    <span>{isRecent ? '上次读到这里' : '双击开始阅读'}</span>
                                </span>
                            </span>
                            {isRecent && <span className="reading-badge">在读</span>}
                        </button>
                        <button className="book-delete" aria-label={'删除 ' + title} title="删除书籍"
                                onClick={() => DeleteFile(name).then(res => {
                                    if (!props.hasError(res)) {
                                        message.info('删除成功！');
                                        props.reloadBookList();
                                    }
                                })}><DeleteOutlined/></button>
                    </li>
                })}
                {!books.length && <li className="library-empty">
                    <BookOutlined/>
                    <h2>留一处安静的书架</h2>
                    <p>添加一本书，或将文件拖入窗口。</p>
                    <Button onClick={props.clickBookPlus}>添加第一本书</Button>
                </li>}
            </ul>
            <footer className="library-footer">
                <span className="library-quote" title={sc}>{sc || '翻开一页，暂别喧嚣。'}</span>
                <button className="footer-icon" title="项目主页" aria-label="项目主页"
                        onClick={() => Browser.OpenURL('https://github.com/lbk-ones/wd-reader')}><GithubOutlined/></button>
                <button className="footer-icon" title="应用设置" aria-label="应用设置"
                        onClick={() => props.setState({sysSettingVisible: true})}><SettingOutlined/></button>
            </footer>
        </section>
    ) : null;
}

export default forwardRef(BookList)
