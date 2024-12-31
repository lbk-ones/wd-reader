import React, {useState, useEffect, useRef} from 'react';
import "./ContextMenu.css"

const ContextMenu = ({options, onSelect,fontSize}) => {
    const [showMenu, setShowMenu] = useState(false);
    const [menuPosition, setMenuPosition] = useState({x: 0, y: 0});
    const menuRef = useRef(null);

    useEffect(() => {
        const handleContextMenu = (e) => {
            e.preventDefault();
            setShowMenu(true);
            setMenuPosition({x: e.clientX, y: e.clientY});
        };

        const handleClick = (e) => {
            e.preventDefault();
            e.stopPropagation();
            setShowMenu(false);
            // console.log('点击。。。。')
        };
        let parentNode = menuRef.current.parentNode ? menuRef.current.parentNode : document;
        parentNode.addEventListener('contextmenu', handleContextMenu);
        document.addEventListener('click', handleClick);

        return () => {
            parentNode.removeEventListener('contextmenu', handleContextMenu);
            parentNode.removeEventListener('click', handleClick);
        };
    }, []);

    return (
        <div
            ref={menuRef}
            style={{
                display: showMenu ? 'block' : 'none',
                position: 'fixed',
                left: `${menuPosition.x}px`,
                top: `${menuPosition.y}px`,
                backgroundColor: 'white',
                border: '1px solid #ccc',
                borderRadius: '4px',
                boxShadow: '0 2px 4px rgba(0, 0, 0, 0.2)',
                zIndex: 1000,
                width: '100px'
            }}
        >
            {options.map((option, index) => (
                <div
                    key={index}
                    title={option.label}
                    className={'context-optional'}
                    style={{
                        cursor: 'pointer',
                        color: '#000',
                        width: '100%',
                        height: '100%',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                        textAlign: 'center',
                        textIndent: 0,
                        ...(function (){

                            if(fontSize){
                                return  {
                                    fontSize
                                }
                            }
                            return {

                            }
                        })()
                    }}
                    onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        onSelect(option);
                        setShowMenu(false);
                    }}
                >
                    {option.label}
                </div>
            ))}
        </div>
    );
};

export default ContextMenu;