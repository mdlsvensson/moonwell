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

    /// <summary>The analyzer's diagnostics for a whole file, in source order, each as "id: the code it points at"; path names the file.</summary>
    public static List<string> CheckFile(string source, string path = "")
    {
        return Found(Analyze(source, path: path));
    }

    /// <summary>The id analyzer's diagnostics for a whole file, in source order, each as "id: the code it points at".</summary>
    public static List<string> CheckIds(string source)
    {
        return Found(Analyze(source, analyzers: ImmutableArray.Create<DiagnosticAnalyzer>(new IdAnalyzer())));
    }

    /// <summary>The messages of the id analyzer's diagnostics for a whole file, in source order.</summary>
    public static List<string> MessagesOfIds(string source)
    {
        return Analyze(source, analyzers: ImmutableArray.Create<DiagnosticAnalyzer>(new IdAnalyzer())).Diagnostics.OrderBy(d => d.Location.SourceSpan.Start).Select(d => d.GetMessage()).ToList();
    }

    /// <summary>The messages of the analyzer's diagnostics for a whole file, in source order.</summary>
    public static List<string> MessagesOfFile(string source)
    {
        return Analyze(source).Diagnostics.OrderBy(d => d.Location.SourceSpan.Start).Select(d => d.GetMessage()).ToList();
    }

    /// <summary>
    /// The analyzer's diagnostics for a whole file that does not compile, in source order, each as "id: the code it points at";
    /// its compiler errors are left to the caller.
    /// </summary>
    public static List<string> CheckBrokenFile(string source)
    {
        return Found(Analyze(source, broken: true));
    }

    /// <summary>The messages of the analyzer's diagnostics for a whole file that does not compile, in source order.</summary>
    public static List<string> MessagesOfBrokenFile(string source)
    {
        return Analyze(source, broken: true).Diagnostics.OrderBy(d => d.Location.SourceSpan.Start).Select(d => d.GetMessage()).ToList();
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

    /// <summary>Compiles a whole file and loads it, to run the map code on .NET as the build does.</summary>
    public static System.Reflection.Assembly Load(string source)
    {
        var (_, compilation) = Compile(source);
        using var stream = new MemoryStream();
        var emitted = compilation.Emit(stream);
        Assert.True(emitted.Success, string.Join("\n", emitted.Diagnostics));
        return System.Reflection.Assembly.Load(stream.ToArray());
    }

    private static (SyntaxTree Tree, CSharpCompilation Compilation) Compile(string source, bool broken = false, string path = "")
    {
        var tree = CSharpSyntaxTree.ParseText(source, new CSharpParseOptions(LanguageVersion.Latest), path);
        var compilation = CSharpCompilation.Create("Map" + Guid.NewGuid().ToString("N"), new[] { tree }, References,
            new CSharpCompilationOptions(OutputKind.DynamicallyLinkedLibrary, nullableContextOptions: NullableContextOptions.Enable));
        var errors = compilation.GetDiagnostics().Where(d => d.Severity == DiagnosticSeverity.Error).ToList();
        Assert.True(broken ? errors.Count > 0 : errors.Count == 0, string.Join("\n", errors));
        return (tree, compilation);
    }

    // Every analyzer but the id analyzer, unless the caller names them: the other tests' classes have no ids.
    private static (SyntaxTree Tree, ImmutableArray<Diagnostic> Diagnostics) Analyze(string source, bool broken = false, ImmutableArray<DiagnosticAnalyzer>? analyzers = null, string path = "")
    {
        var (tree, compilation) = Compile(source, broken, path);
        analyzers ??= ImmutableArray.Create<DiagnosticAnalyzer>(new SyncAnalyzer(), new LuaAnalyzer(), new SyntaxAnalyzer(), new FieldRulesAnalyzer(), new MovedAnalyzer(), new DeclaredAnalyzer(), new ReactionAnalyzer(), new ScopeAnalyzer());
        return (tree, compilation.WithAnalyzers(analyzers.Value).GetAnalyzerDiagnosticsAsync().Result);
    }
}
