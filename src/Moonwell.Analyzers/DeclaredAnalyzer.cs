using System.Collections.Generic;
using System.Collections.Immutable;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Diagnostics;

namespace Moonwell.Analyzers;

/// <summary>
/// Rejects the game's live values and actions in the constructor of a declared object class (MW0012): a class that
/// inherits <c>StandardUnit</c>, <c>StandardHero</c> or <c>StandardItem</c>, whose constructor the build runs with no game.
/// A member of <c>Unit</c>, <c>Hero</c> or <c>Item</c> on the object itself is reported, except <c>Components</c>; code
/// in a lambda or a local function may run later in the game, and is left alone.
/// </summary>
[DiagnosticAnalyzer(LanguageNames.CSharp)]
public sealed class DeclaredAnalyzer : DiagnosticAnalyzer
{
    // The declared field to set in place of a live value of the same meaning.
    private static readonly Dictionary<string, string> DeclaredFields = new()
    {
        ["Mana"] = "ManaPoints",
        ["MaxMana"] = "ManaPoints",
        ["Life"] = "HitPoints",
        ["MaxLife"] = "HitPoints",
        ["Scale"] = "ScalingValue",
        ["Strength"] = "StartingStrength",
        ["Agility"] = "StartingAgility",
        ["Intelligence"] = "StartingIntelligence",
        ["Charges"] = "NumberOfCharges",
    };

    public override ImmutableArray<DiagnosticDescriptor> SupportedDiagnostics => Rules.Declared;

    public override void Initialize(AnalysisContext context)
    {
        context.ConfigureGeneratedCodeAnalysis(GeneratedCodeAnalysisFlags.None);
        context.EnableConcurrentExecution();
        context.RegisterSyntaxNodeAction(Constructor, SyntaxKind.ConstructorDeclaration);
    }

    private static void Constructor(SyntaxNodeAnalysisContext context)
    {
        var constructor = (ConstructorDeclarationSyntax)context.Node;
        if (constructor.Modifiers.Any(SyntaxKind.StaticKeyword))
        {
            return;
        }

        var type = context.SemanticModel.GetDeclaredSymbol(constructor, context.CancellationToken)?.ContainingType;
        if (type == null || !IsDeclared(type))
        {
            return;
        }

        var body = (SyntaxNode?)constructor.Body ?? constructor.ExpressionBody;
        if (body == null)
        {
            return;
        }

        foreach (var node in body.DescendantNodes(child => child is not AnonymousFunctionExpressionSyntax and not LocalFunctionStatementSyntax))
        {
            if (node is SimpleNameSyntax name)
            {
                Check(context, name);
            }
        }
    }

    private static void Check(SyntaxNodeAnalysisContext context, SimpleNameSyntax name)
    {
        // Only a member of the object itself: Mana, this.Mana, base.Mana; not other.Mana or x?.Mana.
        if (name.Parent is MemberAccessExpressionSyntax access && access.Name == name)
        {
            if (access.Expression is not ThisExpressionSyntax and not BaseExpressionSyntax)
            {
                return;
            }
        }
        else if (name.Parent is MemberBindingExpressionSyntax or QualifiedNameSyntax)
        {
            return;
        }

        var symbol = context.SemanticModel.GetSymbolInfo(name, context.CancellationToken).Symbol;
        if (symbol is not (IPropertySymbol or IMethodSymbol) || symbol.IsStatic || symbol.Name == "Components" || !IsRuntimeType(symbol.ContainingType)
            || InNameOf(context, name))
        {
            return;
        }

        var text = name.Identifier.ValueText;
        var why = DeclaredFields.TryGetValue(text, out var field)
            ? $"is the {(IsNamed(symbol.ContainingType, "Item") ? "item" : "unit")}'s value in the game, which a declared class's constructor cannot reach; the declared field is {field}"
            : "calls the game, which a declared class's constructor cannot do; game work starts from a reaction or the game's start";
        context.ReportDiagnostic(Diagnostic.Create(Rules.LiveInDeclaration, name.GetLocation(), text, why));
    }

    // A class that inherits a standard unit, hero or item of Moonwell.Objects: the build runs its constructor to read its fields.
    private static bool IsDeclared(INamedTypeSymbol type)
    {
        for (var at = type.BaseType; at != null; at = at.BaseType)
        {
            if ((at.Name == "StandardUnit" || at.Name == "StandardHero" || at.Name == "StandardItem")
                && at.ContainingNamespace is { Name: "Objects", ContainingNamespace: { Name: "Moonwell", ContainingNamespace.IsGlobalNamespace: true } })
            {
                return true;
            }
        }

        return false;
    }

    private static bool IsRuntimeType(INamedTypeSymbol? type)
    {
        return IsNamed(type, "Unit") || IsNamed(type, "Hero") || IsNamed(type, "Item");
    }

    private static bool IsNamed(INamedTypeSymbol? type, string name)
    {
        return type is { ContainingNamespace: { Name: "Moonwell", ContainingNamespace.IsGlobalNamespace: true } } && type.Name == name;
    }

    // nameof(Mana) names the member without reading it.
    private static bool InNameOf(SyntaxNodeAnalysisContext context, SyntaxNode node)
    {
        foreach (var ancestor in node.Ancestors())
        {
            if (ancestor is InvocationExpressionSyntax { Expression: IdentifierNameSyntax { Identifier.ValueText: "nameof" } } invocation
                && context.SemanticModel.GetSymbolInfo(invocation, context.CancellationToken).Symbol == null)
            {
                return true;
            }

            if (ancestor is StatementSyntax)
            {
                return false;
            }
        }

        return false;
    }
}
