"use strict";

const EmberApp = require("ember-cli/lib/broccoli/ember-app");
const PostcssCompiler = require('broccoli-postcss');
const Funnel = require('broccoli-funnel');
const mergeTrees = require('broccoli-merge-trees');

module.exports = function (defaults) {
  const tailwindcss = require('tailwindcss');
  const autoprefixer = require('autoprefixer');
  
  let app = new EmberApp(defaults, {
    // Code coverage configuration
    'ember-cli-code-coverage': {
      modifyAssetLocation: function(path) {
        // Adjust path for coverage reports
        return path.replace('hermes', '');
      }
    },
    // Ember Auto Import configuration
    autoImport: {
      webpack: {
        resolve: {
          alias: {
            // Explicitly resolve @ember/test-waiters for ember-app-scheduler
            '@ember/test-waiters': require.resolve('@ember/test-waiters')
          }
        }
      }
    },
    minifyCSS: {
      enabled: false,
    },
    postcssOptions: {
      compile: {
        extension: "scss",
        enabled: true,
        parser: require("postcss-scss"),
        cacheInclude: [/.*\.hbs$/, /.*\.scss$/],
        plugins: [
          {
            module: require("@csstools/postcss-sass"),
            options: {
              includePaths: [
                // Brings in the styles for TailwindCSS from node_modules
                // - Required so we can use @import to include the styles in SASS
                "node_modules",
                // And the styles from @hashicorp/design-system-tokens
                // - Required for @hashicorp/design-system-components to work
                "./node_modules/@hashicorp/design-system-tokens/dist/products/css",
              ],
            },
          },
          require("tailwindcss")("./tailwind.config.js"),
        ],
      },
    },
  });

  // Get the output tree
  let tree = app.toTree();
  
  // Post-process just the CSS files with Tailwind
  let cssTree = new Funnel(tree, {
    include: ['assets/*.css'],
    destDir: '.'
  });
  
  let processedCss = new PostcssCompiler(cssTree, {
    plugins: [
      {
        module: tailwindcss,
        options: require('./tailwind.config.js')
      },
      {
        module: autoprefixer
      }
    ],
    map: false
  });
  
  // Merge processed CSS back with the rest of the tree
  let otherAssets = new Funnel(tree, {
    exclude: ['assets/*.css']
  });
  
  return mergeTrees([processedCss, otherAssets], { overwrite: true });
};
