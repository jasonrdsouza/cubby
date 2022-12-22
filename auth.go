package main

/* todo
 * - access list struct ([]string?)
 * - add access list to metadata
 * - automatically add creating user to access list
 * - optionally add access list based on header
 *   - special case "*" to be "all users"
 * - add function to extract username from auth header
 * - plumb access check code through server handler
 * - client support for passing auth header value in?
 *   - probably not needed/ false sense of security
 * - readme explanation that we are "outsourcing" auth
 */
