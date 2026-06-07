import { Box, CircularProgress, Container, Typography } from "@mui/material";

export function LoadingScreen() {
  return (
    <Box sx={{ minHeight: "100vh", display: "grid", placeItems: "center" }}>
      <Container maxWidth="sm" sx={{ display: "grid", placeItems: "center", py: 4 }}>
        <Box sx={{ display: "grid", placeItems: "center", gap: 1.5, textAlign: "center" }}>
          <CircularProgress size={28} thickness={5} />
          <Typography variant="h6" sx={{ fontWeight: 700, letterSpacing: "-0.02em" }}>
            DCloud
          </Typography>
        </Box>
      </Container>
    </Box>
  );
}
