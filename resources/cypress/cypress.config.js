const { defineConfig } = require("cypress");
const otplib = require("otplib");

module.exports = defineConfig({
  e2e: {
    setupNodeEvents(on, config) {
      on('task', {
        generateToken(secret) {
          return otplib.authenticator.generate(secret);
        },
        log(message) {
          console.log(message);
          return null;
        },
      })
    },
  },
});
