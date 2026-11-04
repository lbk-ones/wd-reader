import ginkgo from '../assets/reading-backgrounds/ginkgo.png';
import willow from '../assets/reading-backgrounds/willow.png';
import bamboo from '../assets/reading-backgrounds/bamboo.png';

export const READING_BACKGROUND_PRESETS = [
    {id: 'paper', name: '银杏', image: ginkgo},
    {id: 'mist', name: '柳叶', image: willow},
    {id: 'white-bamboo', name: '竹叶', image: bamboo},
];

export function getReadingBackground(id) {
    return READING_BACKGROUND_PRESETS.find(preset => preset.id === id);
}
