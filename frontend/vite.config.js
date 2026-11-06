import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// https://vitejs.dev/config/
export default defineConfig({
    // Wails' development asset proxy connects over IPv4.
    server: {
        host: '127.0.0.1',
        port: Number(process.env.WAILS_VITE_PORT) || 9245,
        strictPort: true,
    },
    build: {
        // Keep Vite 3's browser targets for older desktop WebViews.
        target: ['es2020', 'edge88', 'firefox78', 'chrome87', 'safari14'],
    },
    plugins: [react()],
    // ,
    //     build: {
    //         rollupOptions: {
    //             output: {
    //                 manualChunks(id) {
    //                     if (id.includes('antd')) {
    //                         return 'antd';
    //                     } else if (id.includes('lodash')) {
    //                         return 'lodash';
    //                     } else if (id.includes('node_modules')) {
    //                         return 'vendor';
    //                     }
    //                 },
    //             },
    //         },
    //     },
});
