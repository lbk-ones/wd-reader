import {isEmpty} from "lodash-es";
import CsSVg from "../../assets/images/cs.svg";
import {
    BorderOutlined,
    CloseOutlined,
    DatabaseOutlined,
    DisconnectOutlined,
    MinusOutlined,
    PushpinOutlined,
    RedoOutlined,
    SettingOutlined
} from "@ant-design/icons";
import {setCacheItem} from "../Utils.jsx";
import {
    Quit, WindowCenter,
    WindowGetSize,
    WindowMinimise, WindowReload,
    WindowReloadApp, WindowSetSize,
    WindowToggleMaximise
} from "../../../wailsjs/runtime/runtime.js";
import {memo} from "react";
import * as PropTypes from "prop-types";
import {message} from "antd";



function Header(props) {

    const state = props.state;
    const setState = props.setState;
    const isAlwaysTop = props.isAlwaysTop;
    const setSettingState = props.setSettingState;
    const settingstate = props.settingstate;
    const reloadBookList = props.reloadBookList;
    let isDisplay = props.display && (state.showTitle || isEmpty(state.currentBookName));

    if (!isDisplay) {
        return null
    }

    function getMenuSuffixBtn(type = "0") {

        return (
            <>
                {
                    type === '1' && (
                        <SettingOutlined title={"设置！"}
                                         onClick={() => {
                                             setState({
                                                 settingVisible: true
                                             })
                                         }}/>
                    )
                }

                {
                    type === '1' && (
                        <DatabaseOutlined title={"返回书架！"}
                                          onClick={() => {
                                              reloadBookList(() => {
                                                  setState({
                                                      currentBookChapterName: '',
                                                      currentBookName: '',
                                                      currentBookChapterContent: [],
                                                      currentBookChapterList: [],
                                                      muluVisible: false,
                                                      gotoMuluIndexSearchVisible: false,
                                                      lastSearchMulu: -1,
                                                      lastSearchMuluName: "",
                                                      settingVisible: false
                                                  })

                                              })
                                          }}/>
                    )
                }
                <DisconnectOutlined
                    title={"隐藏自己！"}
                    style={{color: settingstate.leaveWindowHid === '1' ? 'gray' : 'unset'}}
                    onClick={() => {
                        let qf = settingstate.leaveWindowHid === '1' ? '0' : '1';
                        setCacheItem("leaveWindowHid", qf)
                        setSettingState({
                            leaveWindowHid: qf
                        })
                    }}
                />
                <PushpinOutlined
                    title={"窗口置顶"}
                    style={{color: settingstate.isAlwaysTop === '1' ? 'gray' : 'unset'}}
                    onClick={() => {
                        let qf = settingstate.isAlwaysTop === '1' ? '0' : '1';
                        isAlwaysTop(qf)
                        setCacheItem("isAlwaysTop", qf)
                        setSettingState({
                            isAlwaysTop: qf
                        })
                    }}/>
                {
                    type !== '1' && (
                        <RedoOutlined title={"刷新"} onClick={() => {
                            WindowReloadApp()
                        }}/>
                    )
                }
                <MinusOutlined onClick={() => {
                    WindowMinimise();
                }}/>
                <BorderOutlined onClick={()=>{

                    WindowGetSize().then(res=>{
                        console.log('res',res)
                        if(res.w === 400 && res.h === 600){
                            WindowSetSize(960,540)
                        }else{
                            WindowSetSize(400,600)
                        }
                        setTimeout(()=>{
                            WindowCenter()
                        },100)
                    });



                }}/>
                <CloseOutlined onClick={() => {
                    Quit();
                }}/>
            </>

        )
    }

    return (<div className={"title"}>


        {/*index page*/}
        {   isEmpty(state.currentBookName) && (
            <div className={"title-div font-bold flex1"}>
                <div className={"w-100 flex no-shrink justify-b align-item-center"} style={{
                    color: "darkred"
                }}>
                    <div className={"flex flex-auto align-item-center gap2"} style={{
                        "--wails-draggable": "drag"
                    }} onDoubleClick={(e)=>{
                        e.stopPropagation()
                        WindowToggleMaximise()
                    }}>
                        <img src={CsSVg} alt=""/>
                        <span>{props.title}</span>
                        <span>{state.version || ""}</span>
                    </div>

                    <div className={"flex flex-row-nowrap gap3 font-s20"}
                         onClick={(e)=>{
                             e.stopPropagation()
                         }}>
                        {
                            getMenuSuffixBtn()
                        }
                    </div>

                </div>
            </div>
        )
        }


        {
            !isEmpty(state.currentBookName) && <>
                <div className={"title-div font-bold"}
                     style={{"--wails-draggable": "drag", display: "flex", flex: 1}}
                     title={state.currentBookName.replace(".txt", "")}
                     onDoubleClick={(e)=>{
                         e.stopPropagation()
                         WindowToggleMaximise()
                     }}
                >
                    <img src={CsSVg} alt=""/>
                    {
                        `${state.currentBookName.replace(".txt", "")}`
                    }
                </div>

                <div className={"title-div font-bold mr-5"}
                     style={{justifyContent: "flex-end", gap: 3, fontSize: 20}}
                     title={state.currentBookChapterName}>

                    {
                        getMenuSuffixBtn("1")
                    }
                </div>

            </>
        }



    </div>)

}



Header.propTypes = {
    display: PropTypes.bool,
    state: PropTypes.any,
    setState: PropTypes.func,
    settingstate: PropTypes.any,
    title: PropTypes.string,
    reloadBookList: PropTypes.func,
};


/**
 * 应用头
 * @param props
 * @returns {JSX.Element|null}
 */
export default memo(Header)