using System.Collections.Immutable;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Diagnostics;

namespace Moonwell.Analyzers;

/// <summary>
/// Names the new home of a choice enum that left <c>Moonwell.Objects</c> (MW0011): a name that no longer binds, beside the
/// compiler's error. It reports <c>Moonwell.Objects.AttackType</c> (or through an alias of the namespace), an
/// <c>AttackType</c> that binds to nothing, and <c>AttackType.Magic</c> in a standard class of a file without
/// <c>using Moonwell</c>, where <c>AttackType</c> is the property and the compiler's error (CS0176) does not name the enum.
/// </summary>
[DiagnosticAnalyzer(LanguageNames.CSharp)]
public sealed class MovedAnalyzer : DiagnosticAnalyzer
{
    public override ImmutableArray<DiagnosticDescriptor> SupportedDiagnostics => Rules.Moved;

    public override void Initialize(AnalysisContext context)
    {
        context.ConfigureGeneratedCodeAnalysis(GeneratedCodeAnalysisFlags.None);
        context.EnableConcurrentExecution();
        context.RegisterSyntaxNodeAction(Name, SyntaxKind.IdentifierName);
    }

    private static void Name(SyntaxNodeAnalysisContext context)
    {
        var name = (IdentifierNameSyntax)context.Node;
        var text = name.Identifier.ValueText;
        if (text != "AttackType" && text != "DefenseType")
        {
            return;
        }

        // The name as written: Moonwell.Objects.AttackType when it is the right side of a qualified name or member access.
        SyntaxNode written = name;
        ExpressionSyntax? left = null;
        switch (name.Parent)
        {
            case QualifiedNameSyntax qualified when qualified.Right == name:
                written = qualified;
                left = qualified.Left;
                break;
            case MemberAccessExpressionSyntax access when access.Name == name:
                written = access;
                left = access.Expression;
                break;
            case MemberBindingExpressionSyntax:
                return;
        }

        var bound = context.SemanticModel.GetSymbolInfo(written, context.CancellationToken);
        if (bound.Symbol != null)
        {
            // AttackType.Magic in a standard class, in a file without using Moonwell: AttackType is the property, not the enum.
            if (left == null && IsCoreValue(bound.Symbol, text) && name.Parent is MemberAccessExpressionSyntax member && member.Expression == name
                && context.SemanticModel.GetSymbolInfo(member, context.CancellationToken).Symbol == null)
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.MovedChoice, name.GetLocation(), text, text));
            }

            return;
        }

        if (!bound.CandidateSymbols.IsEmpty)
        {
            return;
        }

        if (left != null && !IsObjectsNamespace(context.SemanticModel.GetSymbolInfo(left, context.CancellationToken).Symbol))
        {
            return;
        }

        context.ReportDiagnostic(Diagnostic.Create(Rules.MovedChoice, written.GetLocation(), written.ToString(), text));
    }

    // A property (or other value) that holds the core enum of the name, such as a standard unit's AttackType.
    private static bool IsCoreValue(ISymbol symbol, string name)
    {
        var type = symbol switch
        {
            IPropertySymbol property => property.Type,
            IFieldSymbol field => field.Type,
            ILocalSymbol local => local.Type,
            IParameterSymbol parameter => parameter.Type,
            _ => null,
        };
        return type is INamedTypeSymbol { TypeKind: TypeKind.Enum, ContainingNamespace: { Name: "Moonwell", ContainingNamespace.IsGlobalNamespace: true } } core
            && core.Name == name;
    }

    private static bool IsObjectsNamespace(ISymbol? symbol)
    {
        return symbol is INamespaceSymbol { Name: "Objects", ContainingNamespace: { Name: "Moonwell", ContainingNamespace.IsGlobalNamespace: true } };
    }
}
