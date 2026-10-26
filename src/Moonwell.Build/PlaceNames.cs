using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;

namespace Moonwell.Build
{
    /// <summary>
    /// Checks the World Editor names a map's C# gives a package against the names the map has, so that a misspelt name fails the
    /// build at its line instead of failing when the game runs.
    /// </summary>
    public static class PlaceNames
    {
        /// <summary>
        /// Parses every C# file of the map project (syntax only, as <see cref="LineMarks"/> does; the build folder and the top-level
        /// <c>bin</c> and <c>obj</c> are left out) and looks at each call of <paramref name="method"/> with one string literal. A name
        /// that is not in <paramref name="names"/> fails. An argument that is not a literal is left to the runtime check.
        /// </summary>
        /// <param name="project">The map project.</param>
        /// <param name="method">The call to look for, a type and a member: <c>AreaShape.Region</c>. The call may name the type with a namespace before it.</param>
        /// <param name="names">The names the map has.</param>
        /// <param name="kind">What the names are, as the message says it: <c>region</c>.</param>
        /// <exception cref="BuildException">A call names something the map does not have; the first, by file and line.</exception>
        public static void Check(MapProject project, string method, IReadOnlyList<string> names, string kind)
        {
            var dot = method.LastIndexOf('.');
            var type = method.Substring(0, dot);
            var member = method.Substring(dot + 1);
            var known = new HashSet<string>(names, StringComparer.Ordinal);

            var files = CompiledFiles.Sources(project.Folder, project.BuildFolder)
                .Select(path => (Path: path, Relative: Path.GetRelativePath(project.Folder, path).Replace('\\', '/')))
                .OrderBy(file => file.Relative, StringComparer.Ordinal);
            foreach (var file in files)
            {
                var source = File.ReadAllText(file.Path);
                var wrong = new SortedSet<(int Line, string Name)>();

                // The code CSharp.lua compiles has MOONWELL_LUA, and a debug build DEBUG too; each set is looked at.
                foreach (var symbols in new[] { new[] { "MOONWELL_LUA", "DEBUG" }, new[] { "MOONWELL_LUA" } })
                {
                    var options = CSharpParseOptions.Default.WithLanguageVersion(LanguageVersion.Latest).WithPreprocessorSymbols(symbols);
                    var tree = CSharpSyntaxTree.ParseText(source, options);
                    foreach (var call in tree.GetRoot().DescendantNodes().OfType<InvocationExpressionSyntax>())
                    {
                        if (NameOf(call, type, member) is { } name && !known.Contains(name))
                        {
                            wrong.Add((tree.GetLineSpan(call.Span).StartLinePosition.Line + 1, name));
                        }
                    }
                }

                if (wrong.Count > 0)
                {
                    var (line, name) = wrong.Min;
                    var has = known.Count == 0 ? "none" : string.Join(", ", names);
                    throw new BuildException($"{file.Relative}:{line}: the map has no World Editor {kind} named '{name}'. Its {kind}s: {has}.");
                }
            }
        }

        // The name a call gives: its one argument when it is a string literal, and the call is Type.Member or Namespace.Type.Member.
        private static string? NameOf(InvocationExpressionSyntax call, string type, string member)
        {
            if (call.Expression is not MemberAccessExpressionSyntax access
                || access.Name.Identifier.ValueText != member
                || LastName(access.Expression) != type
                || call.ArgumentList.Arguments.Count != 1
                || call.ArgumentList.Arguments[0].Expression is not LiteralExpressionSyntax literal
                || !literal.IsKind(SyntaxKind.StringLiteralExpression))
            {
                return null;
            }

            return literal.Token.ValueText;
        }

        private static string? LastName(ExpressionSyntax expression)
        {
            return expression switch
            {
                IdentifierNameSyntax name => name.Identifier.ValueText,
                MemberAccessExpressionSyntax access => access.Name.Identifier.ValueText,
                _ => null,
            };
        }
    }
}
