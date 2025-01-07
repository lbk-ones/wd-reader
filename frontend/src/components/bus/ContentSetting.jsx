import {Button, Form, InputNumber, Popover, Switch, Tooltip} from "antd";
import {getCacheItem, setCacheItem} from "../Utils.jsx";
import {SketchPicker} from "react-color";
import {ExclamationCircleOutlined} from "@ant-design/icons";
import * as PropTypes from "prop-types";

function ContentSetting(props) {

    let state = props.state;
    let display = props.display;
    let settingState = props.settingState;
    let settingVisible = state.settingVisible;
    let setState = props.setState;
    let isAlwaysTop = props.isAlwaysTop;
    let setDisplay = props.setDisplay;
    let setSettingState = props.setSettingState;

    if (!(display && settingVisible)) {
        return null
    }

    return (
        <div className="setting-modal">
                            <span className={'setting-modal-close'} onClick={() => {
                                setState({
                                    settingVisible: false
                                })
                            }}>
                                ×
                            </span>
            <div className={"setting-modal-center"} onClick={(e) => {
                e.stopPropagation();
            }}>


                <Form
                    layout={"horizontal"}
                    name="basic"
                    labelCol={{
                        span: 9,
                        xs: 9,
                        sm: 9
                    }}
                    wrapperCol={{
                        span: 15,
                        xs: 15,
                        sm: 15,
                    }}
                    initialValues={{
                        layout: "horizontal",
                    }}
                    // onFinish={onFinish}
                    // onFinishFailed={onFinishFailed}
                    autoComplete="off"
                >
                    <Form.Item
                        label="字体大小"
                        name="字体大小"
                        layout="horizontal"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <InputNumber style={{width: 100}} defaultValue={Number(settingState.fontSize)}
                                     onChange={(value) => {
                                         setCacheItem('fontLineHeight', value)
                                         setSettingState({
                                             fontSize: value
                                         })
                                     }}/>
                    </Form.Item>

                    <Form.Item
                        label="间距"
                        name="间距"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <InputNumber style={{width: 100}}
                                     defaultValue={Number(settingState.fontLineHeight)}
                                     onChange={(value) => {
                                         setCacheItem('fontLineHeight', value)
                                         setSettingState({
                                             fontLineHeight: value
                                         })
                                     }}/>
                    </Form.Item>
                    <Form.Item
                        label="背景色"
                        name="背景色"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Popover zIndex={112} content={() => {
                            return (
                                <SketchPicker
                                    color={settingState.bgColor}
                                    onChange={(color) => {
                                        setCacheItem('bgColor', color.hex)
                                        setSettingState({
                                            bgColor: color.hex
                                        })
                                    }}
                                    styles={{
                                        // picker: {
                                        //     width: '', // 调整宽度
                                        //     height: '50%' // 调整高度
                                        // }
                                        // saturation:'50%'
                                    }}
                                />
                            )
                        }} trigger="click" title="颜色选择">

                            <div
                                style={{
                                    width: '20px',
                                    height: '20px',
                                    // margin: '20px',
                                    backgroundColor: settingState.bgColor
                                }}
                            />

                        </Popover>
                    </Form.Item>
                    <Form.Item
                        label="字体颜色"
                        name="字体颜色"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Popover zIndex={112} content={() => {
                            return (
                                <SketchPicker
                                    color={settingState.fontColor}
                                    onChange={(color) => {
                                        let hex = color.hex;
                                        setCacheItem('fontColor', hex)
                                        setSettingState({
                                            fontColor: hex
                                        })
                                    }}
                                    styles={{
                                        // picker: {
                                        //     width: '', // 调整宽度
                                        //     height: '50%' // 调整高度
                                        // }
                                        // saturation:'50%'
                                    }}
                                />
                            )
                        }} trigger="click" title="颜色选择">

                            <div
                                style={{
                                    width: '20px',
                                    height: '20px',
                                    // margin: '20px',
                                    backgroundColor: settingState.fontColor
                                }}
                            />

                        </Popover>

                    </Form.Item>
                    <Form.Item
                        label="点击翻页"
                        name="clickPage"
                    >
                        <input style={{zIndex: 9000, width: 30, height: 30}} type={'checkbox'}
                               checked={settingState.clickPage === '1'} onChange={e => {
                            let string = e.target.checked === true ? "1" : "0";
                            setCacheItem('clickPage', string)
                            setSettingState({
                                clickPage: string
                            })
                        }}/>
                    </Form.Item>
                    <Form.Item
                        label="显示进度"
                        name="显示进度"
                    >
                        <Switch checked={settingState.showProgress === '1'} onChange={(value) => {
                            let string = value === true ? "1" : "0";
                            setCacheItem('showProgress', string)
                            setSettingState({
                                showProgress: string
                            })
                        }}/>
                    </Form.Item>
                    <Form.Item
                        label="窗口置顶"
                        name="窗口置顶"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Switch checked={settingState.isAlwaysTop === '1'} onChange={(value) => {
                            let string = value === true ? "1" : "0";
                            setCacheItem('isAlwaysTop', string)
                            setSettingState({
                                isAlwaysTop: string
                            })
                            isAlwaysTop(string)
                        }}/>
                    </Form.Item>
                    <Form.Item
                        label={<div>窗口隐藏 <Tooltip placement="left"
                                                      title={"鼠标移入的时候才显示，移除变透明"}>
                            <ExclamationCircleOutlined/>
                        </Tooltip></div>}
                        name="窗口隐藏"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Switch checked={settingState.leaveWindowHid === '1'} onChange={(value) => {
                            let string = value === true ? "1" : "0";
                            setCacheItem('leaveWindowHid', string)
                            setSettingState({
                                leaveWindowHid: string
                            })
                            if (string === "0") {
                                setDisplay(true)
                            }
                        }}/>
                    </Form.Item>
                    <Form.Item
                        label={<div>透明模式 <Tooltip placement="left" title={"透明模式会把字体变为白色"}>
                            <ExclamationCircleOutlined/>
                        </Tooltip></div>}
                        name="透明模式"
                        rules={[
                            {
                                required: false,
                                message: 'Please input your username!',
                            },
                        ]}
                    >
                        <Switch checked={settingState.transparentMode === '1'} onChange={(value) => {
                            let string = value === true ? "1" : "0";
                            setCacheItem('transparentMode', string)
                            setSettingState({
                                transparentMode: string
                            })
                            if (string === '1') {
                                setCacheItem('fontColor', '#fff')
                                setSettingState({
                                    fontColor: "#fff"
                                })
                            }
                        }}/>
                    </Form.Item>

                    <Form.Item label={null}>
                        <Button type="primary" onClick={() => {
                            setCacheItem('fontColor', '#000')
                            setCacheItem('fontLineHeight', '30')
                            setCacheItem('bgColor', '#fff')
                            setCacheItem('fontSize', '16')
                            setCacheItem('clickPage', "1")
                            setCacheItem('showProgress', "0")
                            setCacheItem('isAlwaysTop', "1")
                            setCacheItem('transparentMode', "0")
                            setCacheItem('leaveWindowHid', "0")
                            setSettingState({
                                fontColor: "#000",
                                fontLineHeight: "30",
                                bgColor: "#fff",
                                fontSize: "16",
                                clickPage: "1",
                                showProgress: "0",
                                isAlwaysTop: "1",
                                transparentMode: "0",
                                leaveWindowHid: "0",
                            })
                            setDisplay(true)
                            isAlwaysTop(getCacheItem('isAlwaysTop'));
                        }}>
                            恢复默认
                        </Button>
                    </Form.Item>
                </Form>

            </div>
        </div>
    )

}

ContentSetting.propTypes = {

    setState: PropTypes.func,
    state: PropTypes.any,
    settingState: PropTypes.any,
    display: PropTypes.any,
    isAlwaysTop: PropTypes.func,
    setDisplay: PropTypes.func,
    setSettingState: PropTypes.func,
};
export default ContentSetting