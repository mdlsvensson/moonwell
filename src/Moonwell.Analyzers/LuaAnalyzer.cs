using System.Collections.Immutable;
using System.Linq;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Diagnostics;

namespace Moonwell.Analyzers;

/// <summary>Rejects the C# that CSharp.lua drops, stops on or writes wrong, so the editor and dotnet build show it with its line.</summary>
[DiagnosticAnalyzer(LanguageNames.CSharp)]
public sealed class LuaAnalyzer : DiagnosticAnalyzer
{
    public override ImmutableArray<DiagnosticDescriptor> SupportedDiagnostics => Rules.Lua;

    public override void Initialize(AnalysisContext context)
    {
        context.ConfigureGeneratedCodeAnalysis(GeneratedCodeAnalysisFlags.None);
        context.EnableConcurrentExecution();
        context.RegisterSyntaxNodeAction(Using, SyntaxKind.UsingDirective);
        context.RegisterSyntaxNodeAction(Invocation, SyntaxKind.InvocationExpression);
        context.RegisterSyntaxNodeAction(Joined, SyntaxKind.AddExpression);
        context.RegisterSyntaxNodeAction(JoinedByAssignment, SyntaxKind.AddAssignmentExpression);
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

        if (method.Name == "ToString" && !method.IsStatic && ToStringReceiver(invocation) is var (receiver, reported))
        {
            var type = Choice(context.SemanticModel.GetTypeInfo(receiver, context.CancellationToken).Type);
            if (type != null)
            {
                Report(context, reported, type);
                return;
            }
        }

        if (method.IsStatic && method.ContainingType?.SpecialType == SpecialType.System_Enum && TypeMethods.Contains(method.Name))
        {
            var choice = method.TypeArguments.Select(Choice).FirstOrDefault(type => type != null);
            foreach (var argument in invocation.ArgumentList.Arguments)
            {
                if (choice != null)
                {
                    break;
                }

                choice = Choice(argument.Expression is TypeOfExpressionSyntax typeOf
                    ? context.SemanticModel.GetTypeInfo(typeOf.Type, context.CancellationToken).Type
                    : context.SemanticModel.GetTypeInfo(argument.Expression, context.CancellationToken).Type);
            }

            if (choice != null)
            {
                Report(context, invocation, choice);
                return;
            }
        }

        Boxed(context, invocation, method);
    }

    // The receiver of a ToString call and the node to report: x.ToString() and x?.ToString(), where the receiver is the conditional access's.
    private static (ExpressionSyntax Receiver, SyntaxNode Reported)? ToStringReceiver(InvocationExpressionSyntax invocation)
    {
        if (invocation.Expression is MemberAccessExpressionSyntax access)
        {
            return (access.Expression, invocation);
        }

        if (invocation.Expression is MemberBindingExpressionSyntax && invocation.Parent is ConditionalAccessExpressionSyntax conditional
            && conditional.WhenNotNull == invocation)
        {
            return (conditional.Expression, conditional);
        }

        return null;
    }

    // string.Format("{0}", choice), string.Concat("a", choice), builder.Append(choice): an enum given to an object parameter is
    // made into text by the callee's ToString. Equals compares and shows nothing.
    private static void Boxed(SyntaxNodeAnalysisContext context, InvocationExpressionSyntax invocation, IMethodSymbol method)
    {
        if (method.Name is "Equals" or "ReferenceEquals")
        {
            return;
        }

        foreach (var argument in invocation.ArgumentList.Arguments)
        {
            var info = context.SemanticModel.GetTypeInfo(argument.Expression, context.CancellationToken);
            var type = Choice(info.Type);
            if (type != null && info.ConvertedType?.SpecialType == SpecialType.System_Object)
            {
                Report(context, argument.Expression, type);
            }
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
            var type = Choice(context.SemanticModel.GetTypeInfo(side, context.CancellationToken).Type);
            if (type != null)
            {
                Report(context, side, type);
            }
        }
    }

    // text += choice
    private static void JoinedByAssignment(SyntaxNodeAnalysisContext context)
    {
        var assignment = (AssignmentExpressionSyntax)context.Node;
        if (context.SemanticModel.GetTypeInfo(assignment.Left, context.CancellationToken).Type?.SpecialType != SpecialType.System_String)
        {
            return;
        }

        var type = Choice(context.SemanticModel.GetTypeInfo(assignment.Right, context.CancellationToken).Type);
        if (type != null)
        {
            Report(context, assignment.Right, type);
        }
    }

    private static void Interpolated(SyntaxNodeAnalysisContext context)
    {
        var hole = (InterpolationSyntax)context.Node;
        var type = Choice(context.SemanticModel.GetTypeInfo(hole.Expression, context.CancellationToken).Type);
        if (type != null)
        {
            Report(context, hole, type);
        }
    }

    private static void Report(SyntaxNodeAnalysisContext context, SyntaxNode node, ITypeSymbol choice)
    {
        context.ReportDiagnostic(Diagnostic.Create(Rules.ChoiceNames, node.GetLocation(), node.ToString(), choice.Name));
    }

    // The enum of namespace Moonwell.Objects a type is, or the enum under a nullable of it: a choice of World Editor's, which CSharp.lua ignores.
    private static INamedTypeSymbol? Choice(ITypeSymbol? type)
    {
        if (type is INamedTypeSymbol { OriginalDefinition.SpecialType: SpecialType.System_Nullable_T } nullable)
        {
            type = nullable.TypeArguments[0];
        }

        return type is INamedTypeSymbol
        {
            TypeKind: TypeKind.Enum,
            ContainingNamespace: { Name: "Objects", ContainingNamespace: { Name: "Moonwell", ContainingNamespace.IsGlobalNamespace: true } },
        } choice ? choice : null;
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
