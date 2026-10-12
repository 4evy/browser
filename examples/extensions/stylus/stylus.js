const call = async (path, ...args) => {
  const response = await chrome.runtime.sendMessage({
    data: { method: "invokeAPI", path, args },
    TDM: 1,
  });
  if (!response || response.error)
    throw new Error(
      response?.error?.message || "Stylus API returned no response",
    );
  return response.data;
};
const result = await call("styles.importMany", input.files.styles);
if (
  result.length !== input.files.styles.length ||
  result.some((item) => item.err)
)
  throw new Error("Stylus could not import all styles");
await call("styles.setOrder", {
  main: input.files.styles.map((style) => style._id),
  prio: [],
});
const actual = await call("styles.getAll");
for (const wanted of input.files.styles) {
  const found = actual.find((style) => style._id === wanted._id);
  if (!found || found.enabled !== wanted.enabled || !found.sections?.length)
    throw new Error("Stylus did not persist sample " + wanted._id);
}
