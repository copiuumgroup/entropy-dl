import { themeFromSourceColor, argbFromHex, hexFromArgb } from '@material/material-color-utilities';
const theme = themeFromSourceColor(argbFromHex('#32A852'));
console.log(Object.keys(theme.schemes.dark.toJSON()));
