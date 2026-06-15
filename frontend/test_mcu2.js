import { SchemeTonalSpot, Hct, argbFromHex, hexFromArgb } from '@material/material-color-utilities';
const hct = Hct.fromInt(argbFromHex('#32A852'));
const scheme = new SchemeTonalSpot(hct, true, 0.0);
console.log(hexFromArgb(scheme.surfaceContainer));
