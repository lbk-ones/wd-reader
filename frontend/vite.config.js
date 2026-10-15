import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
    // Wails' development asset proxy connects over IPv4.
    server: {
        host: '127.0.0.1',
        port: Number(process.env.WAILS_VITE_PORT) || 9245,
        strictPort: true,
    },
    plugins: [
        react(),
        {
            name: 'wails-dev-readiness',
            configureServer(server) {
                // Wails 3's startup probe has no Accept header. Vite 3's SPA
                // fallback returns 404 for it, preventing the native app launch.
                // Answer only that probe; browsers keep the normal HTML/HMR flow.
                server.middlewares.use((req, res, next) => {
                    if (req.method === 'GET' && req.url === '/' && !req.headers.accept) {
                        res.writeHead(200, {'Content-Type': 'text/plain'});
                        res.end('ready\n');
                        return;
                    }
                    next();
                });
            }
        }
    ]
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
})
