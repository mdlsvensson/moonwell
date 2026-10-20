using System.Collections.Immutable;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Diagnostics;

namespace Moonwell.Analyzers;

/// <summary>Rejects the C# that CSharp.lua drops or stops on, so the editor and dotnet build show it with its line.</summary>
[DiagnosticAnalyzer(LanguageNames.CSharp)]
public sealed class LuaAnalyzer : DiagnosticAnalyzer
{
    public override ImmutableArray<DiagnosticDescriptor> SupportedDiagnostics => Rules.Lua;

    public override void Initialize(AnalysisContext context)
    {
        context.ConfigureGeneratedCodeAnalysis(GeneratedCodeAnalysisFlags.None);
        context.EnableConcurrentExecution();
        context.RegisterSyntaxNodeAction(
            node => node.ReportDiagnostic(Diagnostic.Create(Rules.CollectionExpression, node.Node.GetLocation())),
            SyntaxKind.CollectionExpression);
        context.RegisterSyntaxNodeAction(Using, SyntaxKind.UsingDirective);
    }

    private static void Using(SyntaxNodeAnalysisContext context)
    {
        var directive = (UsingDirectiveSyntax)context.Node;
        if (directive.StaticKeyword.IsKind(SyntaxKind.None) || directive.Name == null)
        {
            return;
        }

        var imported = context.SemanticModel.GetSymbolInfo(directive.Name, context.CancellationToken).Symbol;
        if (imported is INamedTypeSymbol { Name: "Natives", ContainingNamespace: { Name: "Moonwell", ContainingNamespace.IsGlobalNamespace: true } })
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.StaticNatives, directive.GetLocation()));
        }
    }
}
