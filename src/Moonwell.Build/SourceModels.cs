using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;

namespace Moonwell.Build
{
    /// <summary>The syntax trees and semantic models of a map project's sources, compiled together as Roslyn reads them.</summary>
    internal static class SourceModels
    {
        // Each file's tree and semantic model, in a compilation of every source.
        internal static IEnumerable<(SyntaxTree Tree, SemanticModel Model)> Of(
            IEnumerable<string> sources,
            IEnumerable<string> files,
            string assemblyName,
            IEnumerable<string> references,
            IEnumerable<string> symbols)
        {
            var options = CSharpParseOptions.Default.WithLanguageVersion(LanguageVersion.Latest).WithPreprocessorSymbols(symbols);
            var trees = sources
                .Select(path => CSharpSyntaxTree.ParseText(File.ReadAllText(path), options, Path.GetFullPath(path)))
                .ToList();
            var libraries = references.Concat(FrameworkLibraries())
                .Distinct(StringComparer.OrdinalIgnoreCase)
                .Select(path => MetadataReference.CreateFromFile(path));
            var compilation = CSharpCompilation.Create(
                assemblyName,
                trees,
                libraries,
                new CSharpCompilationOptions(OutputKind.DynamicallyLinkedLibrary, allowUnsafe: true));

            var byPath = trees.ToDictionary(tree => tree.FilePath, StringComparer.OrdinalIgnoreCase);
            foreach (var file in files)
            {
                var tree = byPath[Path.GetFullPath(file)];
                yield return (tree, compilation.GetSemanticModel(tree));
            }
        }

        // The libraries of the .NET the build runs on, which is the one the map targets.
        private static IEnumerable<string> FrameworkLibraries()
        {
            var runtime = Path.GetDirectoryName(typeof(object).Assembly.Location)!;
            var trusted = (string?)AppContext.GetData("TRUSTED_PLATFORM_ASSEMBLIES") ?? string.Empty;
            return trusted.Split(Path.PathSeparator, StringSplitOptions.RemoveEmptyEntries)
                .Where(path => string.Equals(Path.GetDirectoryName(path), runtime, StringComparison.OrdinalIgnoreCase));
        }
    }
}
