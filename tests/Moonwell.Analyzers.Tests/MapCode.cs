using System.Collections.Immutable;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.Diagnostics;
using Moonwell.Analyzers;

namespace Moonwell.Analyzers.Tests;

/// <summary>Compiles a piece of map code against Moonwell and runs the analyzer on it.</summary>
internal static class MapCode
{
    private static readonly IReadOnlyList<MetadataReference> References =
        ((string)AppContext.GetData("TRUSTED_PLATFORM_ASSEMBLIES")!)
            .Split(Path.PathSeparator)
            .Select(path => (MetadataReference)MetadataReference.CreateFromFile(path))
            .ToList();

    /// <summary>
    /// The analyzer's diagnostics for a body run with a player p and a unit u, and members beside it, in source
    /// order, each as "id: the code it points at".
    /// </summary>
    public static List<string> Check(string body, string members = "")
    {
        return Found(Analyze(Wrap(body, members)));
    }

    /// <summary>The analyzer's diagnostics for a whole file, in source order, each as "id: the code it points at".</summary>
    public static List<string> CheckFile(string source)
    {
        return Found(Analyze(source));
    }

    /// <summary>The messages of the analyzer's diagnostics for a whole file, in source order.</summary>
    public static List<string> MessagesOfFile(string source)
    {
        return Analyze(source).Diagnostics.OrderBy(d => d.Location.SourceSpan.Start).Select(d => d.GetMessage()).ToList();
    }

    private static List<string> Found((SyntaxTree Tree, ImmutableArray<Diagnostic> Diagnostics) analyzed)
    {
        var (tree, diagnostics) = analyzed;
        return diagnostics
            .OrderBy(d => d.Location.SourceSpan.Start)
            .Select(d => d.Id + ": " + tree.GetText().ToString(d.Location.SourceSpan))
            .ToList();
    }

    /// <summary>The message of the only diagnostic for a body and members.</summary>
    public static string Message(string body, string members = "")
    {
        return Assert.Single(Analyze(Wrap(body, members)).Diagnostics).GetMessage();
    }

    private static string Wrap(string body, string members)
    {
        return $$"""
            using System;
            using System.Collections.Generic;
            using System.Linq;
            using Moonwell;

            namespace Map
            {
                public static class Code
                {
                    public static void Run(Player p, unit u)
                    {
            {{body}}
                    }

            {{members}}
                }
            }
            """;
    }

    private static (SyntaxTree Tree, ImmutableArray<Diagnostic> Diagnostics) Analyze(string source)
    {
        var tree = CSharpSyntaxTree.ParseText(source, new CSharpParseOptions(LanguageVersion.Latest));
        var compilation = CSharpCompilation.Create("Map", new[] { tree }, References,
            new CSharpCompilationOptions(OutputKind.DynamicallyLinkedLibrary, nullableContextOptions: NullableContextOptions.Enable));
        var errors = compilation.GetDiagnostics().Where(d => d.Severity == DiagnosticSeverity.Error).ToList();
        Assert.True(errors.Count == 0, string.Join("\n", errors));

        var analyzers = ImmutableArray.Create<DiagnosticAnalyzer>(new SyncAnalyzer(), new LuaAnalyzer(), new FieldRulesAnalyzer());
        return (tree, compilation.WithAnalyzers(analyzers).GetAnalyzerDiagnosticsAsync().Result);
    }
}
