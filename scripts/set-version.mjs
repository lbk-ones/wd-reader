import fs from 'node:fs';
import {execFileSync} from 'node:child_process';

const version = process.argv[2]?.replace(/^v/, '');
if (!/^\d+\.\d+(?:\.\d+)?(?:-[0-9A-Za-z.-]+)?$/.test(version || '')) {
    throw new Error('Usage: node scripts/set-version.mjs 1.4.0');
}
const configPath = 'build/config.yml';
const config = fs.readFileSync(configPath, 'utf8');
fs.writeFileSync(configPath, config.replace(/^  version: .*$/m, `  version: "${version}"`));
execFileSync(process.platform === 'win32' ? 'wails3.exe' : 'wails3', ['task', 'common:update:build-assets'], {stdio: 'inherit'});
