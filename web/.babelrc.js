module.exports = {
  presets: [
    '@babel/preset-env'
  ],
  plugins: [
    // Decorators must come first (required by Ember)
    ['@babel/plugin-proposal-decorators', { legacy: true }],
    // NOTE: babel-plugin-ember-template-compilation is automatically added by ember-template-imports addon
    // Do NOT manually add it here as it will conflict with the addon's configuration
    // Add support for JavaScript private methods used by Ember Data 5.7.0
    ['@babel/plugin-proposal-private-methods', { loose: true }],
    // Support for private fields as well
    ['@babel/plugin-proposal-private-property-in-object', { loose: true }],
    ['@babel/plugin-proposal-class-properties', { loose: true }],
    // Transform ember-concurrency async tasks to support arrow functions (v4+)
    ['ember-concurrency/async-arrow-task-transform', {
      // Enables async arrow function support for ember-concurrency tasks
      // This resolves the async arrow function Babel compilation error in v4
    }]
  ],
};