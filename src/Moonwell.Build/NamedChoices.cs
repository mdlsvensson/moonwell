using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp.Syntax;

namespace Moonwell.Build
{
    /// <summary>
    /// The choice enums of <c>Moonwell.Objects</c> the map's source names. The built assembly keeps a reference to an enum only
    /// where the code uses it as a value; a constant (<c>(int)Targets.Air</c>), a <c>case</c> label and <c>nameof</c> are folded
    /// into numbers and text by the compiler, yet CSharp.lua still needs the enum to resolve the name.
    /// </summary>
    internal static class NamedChoices
    {
        /// <summary>The enums the code CSharp.lua compiles names, by looking at its source with a semantic model.</summary>
        /// <param name="project">The map project, whose C# files are read (not <c>bin</c>, <c>obj</c> or <c>build</c>).</param>
        /// <param name="assemblyPath">The map's built assembly; the libraries it references are beside it.</param>
        /// <param name="candidates">The names of the enums that are stubbed when named, all in <c>Moonwell.Objects</c>.</param>
        /// <returns>The names of the candidates the source names; empty when it names none.</returns>
        public static IReadOnlySet<string> Read(MapProject project, string assemblyPath, IReadOnlySet<string> candidates)
        {
            var found = new HashSet<string>(StringComparer.Ordinal);
            var sources = Sources(project).ToList();
            if (sources.Count == 0 || candidates.Count == 0)
            {
                return found;
            }

            var assembly = Path.GetFullPath(assemblyPath);
            var libraries = Directory.EnumerateFiles(Path.GetDirectoryName(assembly)!, "*.dll")
                .Where(path => !string.Equals(Path.GetFullPath(path), assembly, StringComparison.OrdinalIgnoreCase))
                .ToList();

            // CSharp.lua compiles with MOONWELL_LUA, and a debug build with DEBUG too; the stubs serve both kinds of build, so code
            // that only one of them has is looked at with its own symbols.
            var symbolSets = new List<string[]> { new[] { "MOONWELL_LUA", "DEBUG" } };
            if (sources.Any(path => File.ReadAllText(path).Contains("DEBUG", StringComparison.Ordinal)))
            {
                symbolSets.Add(new[] { "MOONWELL_LUA" });
            }

            foreach (var symbols in symbolSets)
            {
                foreach (var (tree, model) in FloatLiterals.Models(sources, sources, Path.GetFileNameWithoutExtension(assembly), libraries, symbols))
                {
                    foreach (var name in tree.GetRoot().DescendantNodes().OfType<IdentifierNameSyntax>())
                    {
                        if (candidates.Contains(name.Identifier.ValueText)
                            && EnumOf(model.GetSymbolInfo(name).Symbol) is { } symbol
                            && candidates.Contains(symbol.Name)
                            && symbol.ContainingNamespace.ToDisplayString() == "Moonwell.Objects")
                        {
                            found.Add(symbol.Name);
                        }
                    }
                }
            }

            return found;
        }

        // The enum a name is, is the enum of the member it names, or is the enum an alias stands for.
        private static INamedTypeSymbol? EnumOf(ISymbol? symbol)
        {
            return symbol switch
            {
                IAliasSymbol alias => EnumOf(alias.Target),
                IFieldSymbol { ContainingType.TypeKind: TypeKind.Enum } field => field.ContainingType,
                INamedTypeSymbol { TypeKind: TypeKind.Enum } type => type,
                _ => null,
            };
        }

        // The files dotnet build compiles: every C# file under the project but the top-level bin and obj, and the build folder.
        private static IEnumerable<string> Sources(MapProject project)
        {
            foreach (var file in Directory.EnumerateFiles(project.Folder, "*.cs", SearchOption.TopDirectoryOnly))
            {
                yield return file;
            }

            foreach (var sub in Directory.EnumerateDirectories(project.Folder))
            {
                var name = Path.GetFileName(sub);
                if (name.Equals("bin", StringComparison.OrdinalIgnoreCase) || name.Equals("obj", StringComparison.OrdinalIgnoreCase)
                    || name.Equals("build", StringComparison.OrdinalIgnoreCase))
                {
                    continue;
                }

                foreach (var file in Directory.EnumerateFiles(sub, "*.cs", SearchOption.AllDirectories))
                {
                    yield return file;
                }
            }
        }
    }
}
