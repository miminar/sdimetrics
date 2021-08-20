Plug 'fatih/vim-go'
PlugInstall
let g:go_fmt_command = 'goimports'
let g:go_fmt_autosave = 1
autocmd FileType  go         setlocal    textwidth=110 sw=4 ts=4 noet
