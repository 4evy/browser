{ schema }:

{
  app_id = schema.required schema.types.nonEmptyStr "Installed Flatpak application ID, such as org.mozilla.firefox.";
  branch = schema.optionalNonEmptyString "Installed branch; omit to use Flatpak's default selection.";
  installation = schema.optionalNonEmptyString ''
    Installation selector: user, system, or a named system installation.
    Omit to let Flatpak find the installed application.
  '';
}
