# Clubs Emailer

Currently, this simply generates a list of unpublished clubs and their associated email addresses.
In the future, we can automate this into followup emails.

## Development

Simply create an `.env` file based on `.env.example`.
To generate the list, run `go run .`.

This creates two files `clubs-users-published.json` and `clubs-users-unpublished.json`
