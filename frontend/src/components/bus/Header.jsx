import {isEmpty} from "lodash-es";
import CsSVg from "../../assets/images/cs.svg";

/**
 * 应用头
 * @param props
 * @returns {JSX.Element|null}
 */
export default function (props) {

    const state = props.state;
    let isDisplay = props.display && (state.showTitle || isEmpty(state.currentBookName));

    if (!isDisplay) {
        return null
    }

    return (<div className={"title"}>

        {/*index page*/}
        {
            !isEmpty(state.currentBookName) && (
                <div className={"title-div font-bold"}
                     style={{"--wails-draggable": "drag", display: "flex", flex: 1}}
                     title={state.currentBookName.replace(".txt", "")}>
                    &nbsp;<img src={CsSVg} alt=""/>&nbsp;
                    {
                        `${state.currentBookName.replace(".txt", "")}`
                    }
                </div>
            )
        }

        {
            !isEmpty(state.currentBookName) && (
                <div className={"title-div font-bold mr-5"}
                     style={{justifyContent: "flex-end", gap: 3, fontSize: 20}}
                     title={state.currentBookChapterName}>

                    {
                        props.menuSuffixBtn
                    }
                </div>
            )
        }

        {/*content page*/}
        {
            isEmpty(state.currentBookName) && (
                <div className={"title-div"} style={{
                    fontWeight: "bold",
                    flex: 1
                }}>
                    <div style={{
                        width: "100%",
                        color: "darkred",
                        display: "flex",
                        flexShrink: 0,
                        justifyContent: "space-between",
                        alignItems: "center"
                    }}>
                        <div style={{
                            "--wails-draggable": "drag",
                            display: "flex",
                            flex: "auto",
                            alignItems: "center",
                            gap: "2px"
                        }}>
                            <img src={CsSVg} alt=""/>
                            <span>{props.title}</span>
                            <span>{state.version || ""}</span>
                        </div>

                        <div style={{display: "flex", flexFlow: "row nowrap", gap: 3, fontSize: 20}}
                             onClick={props.onClick}>
                            {
                                props.menuSuffixBtn1
                            }
                        </div>

                    </div>
                </div>
            )
        }

    </div>)

}