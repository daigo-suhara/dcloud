import { createTheme } from "@mui/material";

// GCP Console風の淡色・高情報密度テーマ。primary は Google blue、
// text は charcoal、tables はコンパクト。
export const theme = createTheme({
  palette: {
    mode: "light",
    primary: { main: "#1a73e8", dark: "#1557b0" },
    secondary: { main: "#0f172a" },
    background: {
      default: "#f8f9fa",
      paper: "#ffffff"
    },
    text: {
      primary: "#202124",
      secondary: "#5f6368"
    },
    divider: "#e8eaed",
    success: { main: "#137333" },
    warning: { main: "#b06000" },
    error: { main: "#c5221f" },
    info: { main: "#1a73e8" }
  },
  shape: { borderRadius: 4 },
  typography: {
    fontFamily: '"Google Sans", "Noto Sans JP", "Roboto", sans-serif',
    h4: { fontSize: 22, fontWeight: 500, letterSpacing: 0 },
    h5: { fontSize: 18, fontWeight: 500, letterSpacing: 0 },
    h6: { fontSize: 15, fontWeight: 500 },
    body2: { fontSize: 13 },
    caption: { fontSize: 12 },
    button: {
      textTransform: "none",
      fontWeight: 500,
      letterSpacing: 0.25
    }
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        body: { margin: 0 },
        a: { color: "inherit", textDecoration: "none" }
      }
    },
    MuiButton: {
      defaultProps: { disableElevation: true },
      styleOverrides: {
        root: { borderRadius: 4, minHeight: 32, paddingLeft: 16, paddingRight: 16 },
        contained: { boxShadow: "none", "&:hover": { boxShadow: "none" } },
        outlined: {
          borderColor: "#dadce0",
          "&:hover": { borderColor: "#5f6368", backgroundColor: "rgba(60,64,67,0.04)" }
        }
      }
    },
    MuiPaper: {
      styleOverrides: {
        root: { backgroundImage: "none" },
        outlined: { borderColor: "#dadce0" }
      }
    },
    MuiTableCell: {
      styleOverrides: {
        root: {
          borderColor: "#e8eaed",
          fontSize: 13,
          padding: "8px 16px"
        },
        head: {
          fontWeight: 500,
          color: "#5f6368",
          backgroundColor: "#fafafa",
          fontSize: 12
        }
      }
    },
    MuiTableRow: {
      styleOverrides: {
        root: {
          "&:hover": { backgroundColor: "rgba(60,64,67,0.04)" }
        }
      }
    },
    MuiChip: {
      styleOverrides: {
        root: { height: 22, fontSize: 12, fontWeight: 500 }
      }
    },
    MuiTextField: {
      defaultProps: { size: "small" }
    },
    // Force a consistent Toolbar min-height across breakpoints so the
    // AppBar bottom border lines up with the DrawerNav header divider.
    MuiToolbar: {
      styleOverrides: {
        root: {
          minHeight: 56,
          "@media (min-width: 600px)": { minHeight: 56 }
        }
      }
    },
    MuiDialog: {
      styleOverrides: {
        paper: {
          borderRadius: 8,
          boxShadow: "0 24px 38px 3px rgba(0,0,0,0.14), 0 9px 46px 8px rgba(0,0,0,0.12), 0 11px 15px -7px rgba(0,0,0,0.2)"
        }
      }
    }
  }
});

// Unified monospace stack. Use everywhere code/IDs/URLs need mono
// alignment so the app never mixes system-mono with SFMono etc.
export const monoFontFamily = '"Roboto Mono", "SFMono-Regular", Menlo, Consolas, monospace';

export const shellBg = { background: "#f8f9fa" } as const;

export const actionLinkColor = "#1a73e8";
export const actionLinkHoverColor = "#1557b0";

export const actionLinkSx = {
  color: actionLinkColor,
  fontWeight: 500,
  textDecoration: "none",
  "&:hover": { color: actionLinkHoverColor, textDecoration: "underline" }
} as const;

export const actionLinkButtonSx = {
  px: 0,
  minWidth: 0,
  ...actionLinkSx
} as const;
