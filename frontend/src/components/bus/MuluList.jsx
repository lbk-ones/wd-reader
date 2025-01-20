import {findIndex} from "lodash-es";
import {Input, InputNumber} from "antd";
import {SearchOutlined} from "@ant-design/icons";
import * as PropTypes from "prop-types";
import classNames from "classnames";
import {memo} from "react";
import {useTrackedEffect} from "ahooks";

/**
 * catlog list
 * @param props
 * @returns {JSX.Element}
 * @constructor
 */
function MuluList(props) {

    // const [currentChapterList, setCurrentChapterList] = useState([])

    // const state = props.state;
    const display = props.display;
    const currentBookChapterList = props.currentBookChapterList;
    const muluVisible = props.muluVisible;
    const currentBookChapterName = props.currentBookChapterName;
    const gotoMuluIndexSearchVisible = props.gotoMuluIndexSearchVisible;

    // useTrackedEffect(
    //     (changes) => {
    //         console.log('Index of changed dependencies: ', changes);
    //     },
    //     [...Object.keys(props)],
    // );

    return (
        <div className="mulu-modal" style={{
            visibility: (display && muluVisible && currentBookChapterName) ? "visible" : "hidden",
            opacity: (display && muluVisible && currentBookChapterName) ? 1 : 0,
        }}>
                <span className={"mulu-modal-close"}>
                    <div style={{fontSize: 16}} className={"flex align-item-center"}>
                        <span>
                        {`${findIndex(currentBookChapterList, e => e === currentBookChapterName) + 1}/${currentBookChapterList.length}`}
                        </span>

                        {
                            gotoMuluIndexSearchVisible === true && (
                                <InputNumber min={1} style={{width: "70%"}} onPressEnter={props.onPressEnter}/>
                            )
                        }

                        {
                            gotoMuluIndexSearchVisible === false && (
                                <SearchOutlined style={{
                                    color: "blue",
                                    fontWeight: "bold"
                                }} onClick={props.onClickSearch}/>
                            )
                        }
                    </div>
                    <span className={"mulu-modal-close-x"} onClick={props.closeDrawer}>×</span>
                </span>
            <div className={"flex"} style={{
                position: "sticky",
                top: 0,
                backgroundColor: "#fff"
            }}>
                <Input.Search allowClear={true} onSearch={props.onSearch} style={{flex: "2"}} placeholder={"搜索"}/>
            </div>
            <div className={"mulu-modal-center"} onClick={(e) => {
                e.stopPropagation()
            }}>
                <ul className={"mulu-modal-center-ul"}>
                    {
                        currentBookChapterList.map((ie, index) => {
                            return <li key={"mulu-modal-center-ul-li" + index}
                                       title={ie}
                                       data-key={'mulu-index-' + index}
                                       data-name={ie}
                                       className={classNames('mulu-modal-center-ul-li', {"mulu-modal-center-ul-li-active": ie === currentBookChapterName})}
                                       onClick={(e) => {
                                           e.stopPropagation();
                                           props.clickMulu(ie, index);
                                       }}
                            >
                                {ie}
                            </li>
                        })
                    }
                </ul>
            </div>
        </div>
    )

}

MuluList.propTypes = {
    display: PropTypes.bool,
    state: PropTypes.any,
    currentBookChapterList: PropTypes.any,
    muluVisible: PropTypes.any,
    currentBookChapterName: PropTypes.any,
    gotoMuluIndexSearchVisible: PropTypes.any,
    onPressEnter: PropTypes.func,
    onClickSearch: PropTypes.func,
    closeDrawer: PropTypes.func,
    onSearch: PropTypes.func,
    clickMulu: PropTypes.func
};


export default memo(MuluList)