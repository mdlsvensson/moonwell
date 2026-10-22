using System.Collections.Generic;
using System.Collections.Immutable;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Diagnostics;
using Microsoft.CodeAnalysis.Operations;

namespace Moonwell.Analyzers;

/// <summary>
/// Rejects the game's live values and actions in the constructor of a declared object class (MW0012): a class that
/// inherits <c>StandardUnit</c>, <c>StandardHero</c> or <c>StandardItem</c>, whose constructor the build runs with no game.
/// A member of <c>Unit</c>, <c>Hero</c> or <c>Item</c> on the object itself is reported, except <c>Components</c> and the
/// live members a declared field shares (<c>MaxMana</c>, <c>Name</c>, <c>Strength</c>, <c>Charges</c>), which there are the
/// class's declared values; a hero's <c>Level</c> (and <c>Set("ulev", ...)</c>) is not one, since a hero starts at level 1.
/// The class's own default object (<c>Default</c>, or <c>Unit.Default&lt;T&gt;()</c> of the class or a class under it) is
/// what the constructor is making, and is reported too. Code in a lambda or a local function may run later in the game,
/// and is left alone. Also MW0015: a unit's <c>Level</c>, which has no setter in the game, is set only in the constructor
/// of its declared class.
/// </summary>
[DiagnosticAnalyzer(LanguageNames.CSharp)]
public sealed class DeclaredAnalyzer : DiagnosticAnalyzer
{
    // The live members a declared field shares, by the runtime class that has them: in a declared class's constructor
    // they are the class's declared values.
    private static readonly Dictionary<string, HashSet<string>> SharedMembers = new()
    {
        ["Unit"] = new HashSet<string> { "MaxLife", "MaxMana", "Mana", "Level", "Scale", "FlyHeight", "Name" },
        ["Hero"] = new HashSet<string> { "Strength", "Agility", "Intelligence" },
        ["Item"] = new HashSet<string> { "Charges" },
    };

    public override ImmutableArray<DiagnosticDescriptor> SupportedDiagnostics => Rules.Declared;

    public override void Initialize(AnalysisContext context)
    {
        context.ConfigureGeneratedCodeAnalysis(GeneratedCodeAnalysisFlags.None);
        context.EnableConcurrentExecution();
        context.RegisterSyntaxNodeAction(Constructor, SyntaxKind.ConstructorDeclaration);
        context.RegisterOperationAction(LevelSet, OperationKind.PropertyReference);
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
            else if (node is InvocationExpressionSyntax invocation)
            {
                CheckClassDefault(context, invocation, type);
                if (IsHero(type))
                {
                    CheckSetLevel(context, invocation);
                }
            }
        }
    }

    private const string DefaultWhy = "reads the class's default object: a class's default object is what its constructor is making; read the field itself";

    // Unit.Default<T>() (Hero.Default<T>(), Item.Default<T>()) of the class, or of a class under it, whose default is made with this constructor.
    private static void CheckClassDefault(SyntaxNodeAnalysisContext context, InvocationExpressionSyntax invocation, INamedTypeSymbol type)
    {
        if (context.SemanticModel.GetSymbolInfo(invocation, context.CancellationToken).Symbol is not IMethodSymbol { Name: "Default", IsStatic: true, TypeArguments.Length: 1 } method
            || !IsNamed(method.ContainingType, "Widget"))
        {
            return;
        }

        for (var at = method.TypeArguments[0] as INamedTypeSymbol; at != null; at = at.BaseType)
        {
            if (SymbolEqualityComparer.Default.Equals(at.OriginalDefinition, type.OriginalDefinition))
            {
                context.ReportDiagnostic(Diagnostic.Create(Rules.LiveInDeclaration, invocation.GetLocation(), invocation.ToString(), DefaultWhy));
                return;
            }
        }
    }

    // Set("ulev", ...) on the object itself in a hero class: the same declared level a hero does not have.
    private static void CheckSetLevel(SyntaxNodeAnalysisContext context, InvocationExpressionSyntax invocation)
    {
        if (invocation.Expression is not (IdentifierNameSyntax or MemberAccessExpressionSyntax { Expression: ThisExpressionSyntax or BaseExpressionSyntax })
            || invocation.ArgumentList.Arguments.Count != 2
            || context.SemanticModel.GetConstantValue(invocation.ArgumentList.Arguments[0].Expression, context.CancellationToken).Value as string != "ulev"
            || context.SemanticModel.GetSymbolInfo(invocation, context.CancellationToken).Symbol is not IMethodSymbol { Name: "Set" } method
            || method.ContainingType is not { Name: "StandardHero" } kind || !IsObjects(kind))
        {
            return;
        }

        context.ReportDiagnostic(Diagnostic.Create(Rules.LiveInDeclaration, invocation.GetLocation(), "Set(\"ulev\")", HeroLevelWhy));
    }

    // MW0015: a unit's Level set anywhere but the constructor of its declared class (or a lambda or local function in it).
    private static void LevelSet(OperationAnalysisContext context)
    {
        var reference = (IPropertyReferenceOperation)context.Operation;
        if (reference.Property is not { Name: "Level" } property || !IsNamed(property.ContainingType, "Unit") || !IsWritten(reference))
        {
            return;
        }

        for (var node = reference.Syntax; node != null; node = node.Parent)
        {
            if (node is ConstructorDeclarationSyntax constructor)
            {
                if (!constructor.Modifiers.Any(SyntaxKind.StaticKeyword)
                    && context.ContainingSymbol is IMethodSymbol { MethodKind: MethodKind.Constructor } symbol && IsDeclared(symbol.ContainingType))
                {
                    return;
                }

                break;
            }

            if (node is MemberDeclarationSyntax)
            {
                break;
            }
        }

        context.ReportDiagnostic(Diagnostic.Create(Rules.LevelOutsideConstructor, reference.Syntax.GetLocation()));
    }

    private static bool IsWritten(IOperation operation) => operation.Parent switch
    {
        IAssignmentOperation assignment => assignment.Target == operation,
        IIncrementOrDecrementOperation => true,
        _ => false,
    };

    private static bool IsObjects(INamedTypeSymbol type) =>
        type.ContainingNamespace is { Name: "Objects", ContainingNamespace: { Name: "Moonwell", ContainingNamespace.IsGlobalNamespace: true } };

    private const string HeroLevelWhy = "is the hero's level in the game: a hero starts at level 1; its level is not declared";

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
        if (symbol is IPropertySymbol { Name: "Default", IsStatic: false } && (IsRuntimeType(symbol.ContainingType) || IsKindBase(symbol.ContainingType)))
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.LiveInDeclaration, name.GetLocation(), name.Identifier.ValueText, DefaultWhy));
            return;
        }

        if (symbol is not (IPropertySymbol or IMethodSymbol) || symbol.IsStatic || symbol.Name == "Components" || !IsRuntimeType(symbol.ContainingType)
            || InNameOf(context, name))
        {
            return;
        }

        var text = name.Identifier.ValueText;
        var hero = IsHero(context.ContainingSymbol?.ContainingType);
        if (symbol is IPropertySymbol && SharedMembers.TryGetValue(symbol.ContainingType.Name, out var shared) && shared.Contains(text)
            && !(hero && text == "Level"))
        {
            return;
        }

        string why;
        if (text == "Level")
        {
            why = HeroLevelWhy;
        }
        else if (text == "Life")
        {
            why = "is the unit's value in the game: a unit starts with full life; set MaxLife";
        }
        else if (text == "Add")
        {
            why = "starts a component in the game, which a declared class's constructor cannot do; use Components.Add(...)";
        }
        else
        {
            why = "calls the game, which a declared class's constructor cannot do; game work starts from a reaction or the game's start";
        }

        context.ReportDiagnostic(Diagnostic.Create(Rules.LiveInDeclaration, name.GetLocation(), text, why));
    }

    // A class that inherits a standard unit, hero or item of Moonwell.Objects: the build runs its constructor to read its fields.
    internal static bool IsDeclared(INamedTypeSymbol type)
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

    // StandardUnit, StandardHero or StandardItem of Moonwell.Objects, whose Default hides the runtime's.
    private static bool IsKindBase(INamedTypeSymbol? type) =>
        type is { Name: "StandardUnit" or "StandardHero" or "StandardItem" } && IsObjects(type);

    // A class that inherits Moonwell's Hero.
    private static bool IsHero(INamedTypeSymbol? type)
    {
        for (var at = type; at != null; at = at.BaseType)
        {
            if (IsNamed(at, "Hero"))
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
