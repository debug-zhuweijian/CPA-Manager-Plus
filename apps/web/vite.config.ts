import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { viteSingleFile } from 'vite-plugin-singlefile';
import path from 'path';
import { execSync } from 'child_process';
import fs from 'fs';

function tryGitDescribe(command: string): string {
  try {
    return execSync(command, {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore']
    }).trim();
  } catch {
    return '';
  }
}

// Get version from environment, git tag, or package.json
function getVersion(): string {
  // 1. Environment variable (set by GitHub Actions)
  if (process.env.VERSION) {
    return process.env.VERSION;
  }

  // 2. Try git tag
  const exactGitTag = tryGitDescribe('git describe --tags --exact-match');
  if (exactGitTag) {
    return exactGitTag;
  }
  const gitTag = tryGitDescribe('git describe --tags');
  if (gitTag) {
    return gitTag;
  }

  // 3. Fall back to package.json version
  try {
    const pkg = JSON.parse(fs.readFileSync(path.resolve(__dirname, 'package.json'), 'utf8'));
    if (pkg.version && pkg.version !== '0.0.0') {
      return pkg.version;
    }
  } catch {
    // package.json not readable
  }

  return 'dev';
}

const isDemoSiteBuild = (mode: string) =>
  mode === 'demo' || process.env.DEMO_SITE === 'true' || process.env.VITE_DEMO_SITE === 'true';

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const demoSite = isDemoSiteBuild(mode);
  const useRealDemoFixtures = demoSite || mode === 'test';

  return {
    plugins: [
      react(),
      viteSingleFile({
        removeViteModuleLoader: true
      })
    ],
    define: {
      __APP_VERSION__: JSON.stringify(getVersion()),
      __DEMO_SITE__: JSON.stringify(demoSite || mode === 'test')
    },
    resolve: {
      alias: [
        {
          find: /^@\/features\/demo\/demoFixtures$/,
          replacement: path.resolve(
            __dirname,
            useRealDemoFixtures
              ? './src/features/demo/demoFixtures.ts'
              : './src/features/demo/demoFixtures.empty.ts'
          )
        },
        {
          find: '@',
          replacement: path.resolve(__dirname, './src')
        }
      ]
    },
    css: {
      modules: {
        localsConvention: 'camelCase',
        generateScopedName: '[name]__[local]___[hash:base64:5]'
      },
      preprocessorOptions: {
        scss: {
          additionalData: `@use "@/styles/variables" as *;\n@use "@/styles/mixins" as *;\n`
        }
      }
    },
    build: {
      target: 'es2020',
      outDir: demoSite ? 'dist-demo' : 'dist',
      assetsInlineLimit: 100000000,
      chunkSizeWarningLimit: 100000000,
      cssCodeSplit: false,
      rolldownOptions: {
        output: {
          codeSplitting: false
        }
      }
    }
  };
});
