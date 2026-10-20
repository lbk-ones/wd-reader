'use client';
import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import {message} from "antd";
import {I18nProvider} from './i18n/index.jsx';
import MiniReaderMenu from './components/MiniReaderMenu.jsx';
const container = document.getElementById('root')

const root = createRoot(container)

message.config({
    maxCount: 1,
    duration: 0.4
});

root.render(
    <React.StrictMode>
        <I18nProvider
            theme={{
                token: {
                    // Seed Token，影响范围大
                    colorPrimary: '#476654',
                    colorText: '#303b34',
                    colorTextSecondary: '#7b837c',
                    colorBgContainer: '#fffefa',
                    colorBorder: '#e3e6de',
                    borderRadius: 8,
                    controlHeight: 34,
                    fontFamily: '"Segoe UI", "Microsoft YaHei", "PingFang SC", sans-serif',
                },
            }}
        >
            {new URLSearchParams(window.location.search).get('view') === 'mini-menu' ? <MiniReaderMenu/> : <App/>}
        </I18nProvider>
    </React.StrictMode>
)
