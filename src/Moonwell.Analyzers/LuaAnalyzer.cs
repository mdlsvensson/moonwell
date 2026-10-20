using System.Collections.Immutable;
using System.Linq;
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
        context.RegisterSyntaxNodeAction(Invocation, SyntaxKind.InvocationExpression);
        context.RegisterSyntaxNodeAction(Joined, SyntaxKind.AddExpression);
        context.RegisterSyntaxNodeAction(Interpolated, SyntaxKind.Interpolation);
    }

    // The methods of System.Enum that read an enum's names or members from its type.
    private static readonly ImmutableHashSet<string> TypeMethods = ImmutableHashSet.Create(
        "GetValues", "GetValuesAsUnderlyingType", "GetNames", "GetName", "IsDefined", "Parse", "TryParse", "Format", "ToObject");

    /// <summary>MW0010: an enum of Moonwell.Objects is ignored by CSharp.lua, so the game's Lua has no names or type of it.</summary>
    private static void Invocation(SyntaxNodeAnalysisContext context)
    {
        var invocation = (InvocationExpressionSyntax)context.Node;
        if (context.SemanticModel.GetSymbolInfo(invocation, context.CancellationToken).Symbol is not IMethodSymbol method)
        {
            return;
        }

        if (method.Name == "ToString" && !method.IsStatic && invocation.Expression is MemberAccessExpressionSyntax access
            && IsChoice(context.SemanticModel.GetTypeInfo(access.Expression, context.CancellationToken).Type))
        {
            Report(context, invocation, context.SemanticModel.GetTypeInfo(access.Expression, context.CancellationToken).Type!);
            return;
        }

        if (!method.IsStatic || method.ContainingType?.SpecialType != SpecialType.System_Enum || !TypeMethods.Contains(method.Name))
        {
            return;
        }

        var choice = method.TypeArguments.FirstOrDefault(IsChoice);
        foreach (var argument in invocation.ArgumentList.Arguments)
        {
            if (choice != null)
            {
                break;
            }

            var type = argument.Expression is TypeOfExpressionSyntax typeOf
                ? context.SemanticModel.GetTypeInfo(typeOf.Type, context.CancellationToken).Type
                : context.SemanticModel.GetTypeInfo(argument.Expression, context.CancellationToken).Type;
            choice = IsChoice(type) ? type : null;
        }

        if (choice != null)
        {
            Report(context, invocation, choice);
        }
    }

    // "attack " + unit.AttackType: the string is made with the enum's ToString.
    private static void Joined(SyntaxNodeAnalysisContext context)
    {
        var add = (BinaryExpressionSyntax)context.Node;
        if (context.SemanticModel.GetTypeInfo(add, context.CancellationToken).Type?.SpecialType != SpecialType.System_String)
        {
            return;
        }

        foreach (var side in new[] { add.Left, add.Right })
        {
            var type = context.SemanticModel.GetTypeInfo(side, context.CancellationToken).Type;
            if (IsChoice(type))
            {
                Report(context, side, type!);
            }
        }
    }

    private static void Interpolated(SyntaxNodeAnalysisContext context)
    {
        var hole = (InterpolationSyntax)context.Node;
        var type = context.SemanticModel.GetTypeInfo(hole.Expression, context.CancellationToken).Type;
        if (IsChoice(type))
        {
            Report(context, hole, type!);
        }
    }

    private static void Report(SyntaxNodeAnalysisContext context, SyntaxNode node, ITypeSymbol choice)
    {
        context.ReportDiagnostic(Diagnostic.Create(Rules.ChoiceNames, node.GetLocation(), node.ToString(), choice.Name));
    }

    // An enum of namespace Moonwell.Objects: a choice of World Editor's, which CSharp.lua ignores.
    private static bool IsChoice(ITypeSymbol? type)
    {
        return type is INamedTypeSymbol
        {
            TypeKind: TypeKind.Enum,
            ContainingNamespace: { Name: "Objects", ContainingNamespace: { Name: "Moonwell", ContainingNamespace.IsGlobalNamespace: true } },
        };
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
