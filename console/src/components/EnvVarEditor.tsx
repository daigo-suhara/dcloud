import AddIcon from "@mui/icons-material/Add";
import DeleteIcon from "@mui/icons-material/Delete";
import { Box, Button, IconButton, TextField } from "@mui/material";
import type { EnvVarEntry } from "../types";
import { monoFontFamily } from "../theme";

type Props = {
  value: EnvVarEntry[];
  onChange: (env: EnvVarEntry[]) => void;
  disabled?: boolean;
  size?: "small" | "medium";
};

export function EnvVarEditor({ value, onChange, disabled, size = "small" }: Props) {
  function update(index: number, patch: Partial<EnvVarEntry>) {
    onChange(value.map((e, i) => (i === index ? { ...e, ...patch } : e)));
  }
  function remove(index: number) {
    onChange(value.filter((_, i) => i !== index));
  }
  function add() {
    onChange([...value, { name: "", value: "" }]);
  }

  return (
    <Box sx={{ display: "grid", gap: 1 }}>
      {value.map((entry, i) => (
        <Box key={i} sx={{ display: "grid", gridTemplateColumns: "1fr 1fr auto", gap: 1, alignItems: "center" }}>
          <TextField
            size={size} placeholder="NAME" value={entry.name}
            onChange={(e) => update(i, { name: e.target.value })}
            disabled={disabled}
            slotProps={{ htmlInput: { autoComplete: "off", autoCorrect: "off", autoCapitalize: "none", spellCheck: false, style: { fontFamily: monoFontFamily } } }}
          />
          <TextField
            size={size} placeholder="value" value={entry.value}
            onChange={(e) => update(i, { value: e.target.value })}
            disabled={disabled}
            slotProps={{ htmlInput: { autoComplete: "off", autoCorrect: "off", spellCheck: false, style: { fontFamily: monoFontFamily } } }}
          />
          <IconButton size="small" onClick={() => remove(i)} disabled={disabled} color="error">
            <DeleteIcon fontSize="small" />
          </IconButton>
        </Box>
      ))}
      <Box>
        <Button size="small" startIcon={<AddIcon />} onClick={add} disabled={disabled} variant="outlined">
          追加
        </Button>
      </Box>
    </Box>
  );
}
