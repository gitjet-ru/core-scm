// bootstrap module must be the first one to be imported, it handles webpack lazy-loading and global errors
import './bootstrap.ts';

// many users expect to use jQuery in their custom scripts (https://docs.gitea.com/administration/customizing-gitea#example-plantuml)
// so load globals (including jQuery) as early as possible
import './globals.ts';

import './webcomponents/index.ts';
import './modules/user-settings.ts'; // templates also need to use localUserSettings in inline scripts
import {onDomReady} from './utils/dom.ts';

onDomReady(async () => {
  // when navigate before the import complete, there will be an error from webpack chunk loader:
  // JavaScript promise rejection: Loading chunk index-domready failed.
  try {
    await import(/* webpackChunkName: "index-domready" */'./index-domready.ts');
  } catch (e) {
    if (e.name === 'ChunkLoadError') {
      console.error('Error loading index-domready:', e);
    } else {
      throw e;
    }
  }
});
