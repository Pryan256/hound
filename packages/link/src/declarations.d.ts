// Allow TypeScript to import CSS module files.
declare module "*.css" {
  const styles: Record<string, string>;
  export default styles;
}
