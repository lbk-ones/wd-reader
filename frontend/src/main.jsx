'use client';
import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import {ConfigProvider, message} from "antd";

const container = document.getElementById('root')

const root = createRoot(container)

message.config({
    maxCount: 1,
    duration: 0.4
});

root.render(
    <React.StrictMode>
        <ConfigProvider
            theme={{
                token: {
                    // Seed Token，影响范围大
                    colorPrimary: '#8B0000',
                    borderRadius: 0,
                },
            }}
        >
            <App/>
        </ConfigProvider>
    </React.StrictMode>
)
