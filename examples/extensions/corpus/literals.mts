import { parse } from "@babel/parser";

// Babel nodes vary across the JavaScript and Flow declarations under review
export type Node = Record<string, any>;

export function parseSource(source: string): Node {
  return parse(source, { sourceType: "module", plugins: ["flow", "jsx"] });
}

export function visit(node: unknown, callback: (node: Node) => void): void {
  if (!node || typeof node !== "object") return;
  if (Array.isArray(node)) {
    for (const child of node) visit(child, callback);
    return;
  }
  const value = node as Node;
  if (typeof value.type === "string") callback(value);
  for (const [key, child] of Object.entries(value)) {
    if (key !== "loc" && key !== "comments" && key !== "tokens")
      visit(child, callback);
  }
}

export function property(node: Node, name: string): Node | undefined {
  return node.properties?.find(
    (member: Node) =>
      member.type === "ObjectProperty" &&
      !member.computed &&
      (member.key.name ?? member.key.value) === name,
  )?.value;
}

// Resolve data declarations only; calls other than JSON.stringify are rejected
export function literal(
  node: Node,
  bindings: Map<string, Node> = new Map(),
  resolving = new Set<string>(),
): any {
  switch (node.type) {
    case "StringLiteral":
    case "NumericLiteral":
    case "BooleanLiteral":
      return node.value;
    case "NullLiteral":
      return null;
    case "TypeCastExpression":
      return literal(node.expression, bindings, resolving);
    case "ArrayExpression":
      return node.elements.map((member: Node | null) =>
        member ? literal(member, bindings, resolving) : null,
      );
    case "ObjectExpression": {
      const result: Record<string, any> = {};
      for (const member of node.properties) {
        if (member.type === "SpreadElement") {
          Object.assign(result, literal(member.argument, bindings, resolving));
        } else if (member.type === "ObjectProperty") {
          const key = member.computed
            ? literal(member.key, bindings, resolving)
            : (member.key.name ?? member.key.value);
          result[key] = literal(member.value, bindings, resolving);
        } else throw new Error(`Non-data object member ${member.type}`);
      }
      return result;
    }
    case "Identifier": {
      if (node.name === "undefined") return undefined;
      const declaration = bindings.get(node.name);
      if (!declaration || resolving.has(node.name))
        throw new Error(`Unresolved identifier ${node.name}`);
      const next = new Set(resolving).add(node.name);
      return literal(declaration, bindings, next);
    }
    case "UnaryExpression": {
      const value = literal(node.argument, bindings, resolving);
      if (node.operator === "-") return -value;
      if (node.operator === "+") return +value;
      if (node.operator === "!") return !value;
      if (node.operator === "void") return undefined;
      throw new Error(`Unsupported unary operator ${node.operator}`);
    }
    case "BinaryExpression": {
      const left = literal(node.left, bindings, resolving);
      const right = literal(node.right, bindings, resolving);
      if (node.operator === "+") return left + right;
      if (node.operator === "-") return left - right;
      if (node.operator === "*") return left * right;
      if (node.operator === "/") return left / right;
      if (node.operator === "%") return left % right;
      throw new Error(`Unsupported binary operator ${node.operator}`);
    }
    case "TemplateLiteral": {
      let value = "";
      for (let index = 0; index < node.quasis.length; index++) {
        value += node.quasis[index].value.cooked;
        if (index < node.expressions.length)
          value += literal(node.expressions[index], bindings, resolving);
      }
      return value;
    }
    case "CallExpression":
      if (
        node.callee.type === "MemberExpression" &&
        node.callee.object.name === "JSON" &&
        node.callee.property.name === "stringify" &&
        node.arguments.length === 1
      ) {
        return JSON.stringify(literal(node.arguments[0], bindings, resolving));
      }
      break;
  }
  throw new Error(`Unsupported data expression ${node.type}`);
}

export function declarations(ast: Node): Map<string, Node> {
  const result = new Map<string, Node>();
  for (let statement of ast.program.body) {
    if (statement.type === "ExportNamedDeclaration")
      statement = statement.declaration;
    if (statement?.type !== "VariableDeclaration") continue;
    for (const declaration of statement.declarations) {
      if (declaration.id.type === "Identifier" && declaration.init)
        result.set(declaration.id.name, declaration.init);
    }
  }
  return result;
}
