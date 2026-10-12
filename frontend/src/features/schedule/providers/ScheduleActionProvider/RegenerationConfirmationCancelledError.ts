export class RegenerationConfirmationCancelledError extends Error {
  constructor() {
    super("Todo regeneration confirmation was cancelled")
    this.name = "RegenerationConfirmationCancelledError"
  }
}
