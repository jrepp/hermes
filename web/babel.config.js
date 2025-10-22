module.exports = {
  presets: [
    '@babel/preset-env'
  ],
  plugins: [
    // Decorators must come first (required by Ember)
    ['@babel/plugin-proposal-decorators', { legacy: true }],
    // Add support for JavaScript private methods used by Ember Data 5.7.0
    ['@babel/plugin-proposal-private-methods', { loose: true }],
    // Support for private fields as well
    ['@babel/plugin-proposal-private-property-in-object', { loose: true }],
    ['@babel/plugin-proposal-class-properties', { loose: true }],
    // Transform ember-concurrency async tasks to support arrow functions
    ['ember-concurrency/lib/babel-plugin-transform-ember-concurrency-async-tasks', {
      // Enables async arrow function support for ember-concurrency tasks
      // This resolves the "ember-concurrency/async-arrow-runtime" module error
    }]
  ],
};