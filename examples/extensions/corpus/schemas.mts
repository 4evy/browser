import { parse } from "@babel/parser";

type Value = Record<string, any>;

// Compare sample objects with interface fields, including optional properties
export function checkSchemas(
  source: string,
  examples: Record<string, Value[]>,
) {
  const ast = parse(source, { sourceType: "module", plugins: ["typescript"] });
  const declarations = new Map<string, any>();
  for (const statement of ast.program.body) {
    let node: any = statement;
    if (node.type === "ExportNamedDeclaration") node = node.declaration;
    if (
      node?.type === "TSInterfaceDeclaration" ||
      node?.type === "TSTypeAliasDeclaration"
    )
      declarations.set(node.id.name, node);
  }
  const result: Record<
    string,
    { declared_fields: string[]; configured_fields: string[] }
  > = {};
  for (const [name, values] of Object.entries(examples)) {
    const declaration = declarations.get(name);
    const members =
      declaration?.body?.body ?? declaration?.typeAnnotation?.members;
    if (!members) throw new Error(`Missing object schema ${name}`);
    const fields: string[] = members
      .map((member: any) => member.key?.name ?? member.key?.value)
      .filter(Boolean)
      .sort();
    const configured = [
      ...new Set(values.flatMap((value) => Object.keys(value))),
    ].sort();
    const missing = fields.filter((key) => !configured.includes(key));
    if (missing.length)
      throw new Error(
        `${name}: missing optional/required fields ${missing.join(", ")}`,
      );
    result[name] = { declared_fields: fields, configured_fields: configured };
  }
  return result;
}
