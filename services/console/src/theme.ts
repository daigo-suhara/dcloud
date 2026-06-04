import { createTheme } from "@mui/material";

export const theme = createTheme({
  palette: {
    mode: "light",
    primary: { main: "#0f172a" },
    background: {
      default: "#f8fafc",
      paper: "#ffffff"
    },
    text: {
      primary: "#0f172a",
      secondary: "#64748b"
    }
  },
  shape: {
    borderRadius: 2
  },
  typography: {
    fontFamily: '"Noto Sans JP", sans-serif',
    button: {
      textTransform: "none",
      fontWeight: 600
    }
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        body: {
          margin: 0
        },
        a: {
          color: "inherit",
          textDecoration: "none"
        }
      }
    },
    MuiButton: {
      defaultProps: {
        disableElevation: true
      },
      styleOverrides: {
        root: {
          borderRadius: 10
        }
      }
    },
    MuiPaper: {
      styleOverrides: {
        root: {
          backgroundImage: "none"
        }
      }
    }
  }
});

export const shellBg = {
  background:
    "radial-gradient(circle at top left, rgba(37, 99, 235, 0.08), transparent 24%), radial-gradient(circle at top right, rgba(15, 23, 42, 0.05), transparent 26%), linear-gradient(180deg, #ffffff 0%, #f8fafc 45%, #eef2f7 100%)"
} as const;

export const actionLinkColor = "#2563eb";
export const actionLinkHoverColor = "#1d4ed8";

export const actionLinkSx = {
  color: actionLinkColor,
  fontWeight: 700,
  textDecoration: "underline",
  textUnderlineOffset: "3px",
  "&:hover": {
    color: actionLinkHoverColor,
    textDecorationThickness: "2px"
  }
} as const;

export const actionLinkButtonSx = {
  px: 0,
  minWidth: 0,
  ...actionLinkSx
} as const;
