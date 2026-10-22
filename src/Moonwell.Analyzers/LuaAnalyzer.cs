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
        context.RegisterSyntaxNodeAction(
            node => node.ReportDiagnostic(Diagnostic.Create(Rules.CollectionExpression, node.Node.GetLocation())),
            SyntaxKind.CollectionExpression);
        context.RegisterSyntaxNodeAction(Using, SyntaxKind.UsingDirective);
        context.RegisterSyntaxNodeAction(Invocation, SyntaxKind.InvocationExpression);
        context.RegisterSyntaxNodeAction(Joined, SyntaxKind.AddExpression);
        context.RegisterSyntaxNodeAction(JoinedByAssignment, SyntaxKind.AddAssignmentExpression);
        context.RegisterSyntaxNodeAction(Interpolated, SyntaxKind.Interpolation);
        context.RegisterSyntaxNodeAction(Conditional, SyntaxKind.ConditionalExpression);
    }

    /// <summary>
    /// MW0016: CSharp.lua writes c ? a : b as c and a or b when a's own type is a value type (a bool only when a is the
    /// constant true), so the game gives b when a is nil or false (C24).
    /// </summary>
    private static void Conditional(SyntaxNodeAnalysisContext context)
    {
        var conditional = (ConditionalExpressionSyntax)context.Node;
        var model = context.SemanticModel;
        var whenTrue = Unwrapped(conditional.WhenTrue);
        // Its own type, as CSharp.lua reads it: c ? n : (object)1 is written with and/or for an int? n.
        var type = model.GetTypeInfo(whenTrue, context.CancellationToken).Type;
        if (type == null || !type.IsValueType)
        {
            return;
        }

        bool canBeNil, canBeFalse;
        if (type is INamedTypeSymbol { OriginalDefinition.SpecialType: SpecialType.System_Nullable_T } nullable)
        {
            var plain = Plain(model, whenTrue, context);
            // c ? n : null gives nil when n is nil, as C# gives null.
            canBeNil = plain == null && !Checked(model, conditional.Condition, whenTrue, context) && !IsNull(model, conditional.WhenFalse, context);
            canBeFalse = MayBeBool(nullable.TypeArguments[0])
                && !(plain != null && model.GetConstantValue(plain, context.CancellationToken) is { HasValue: true, Value: true });
        }
        else
        {
            // A plain bool is written with if; another value is never nil or false, unless a type parameter can be a bool.
            canBeNil = false;
            canBeFalse = type is ITypeParameterSymbol;
        }

        if (canBeNil || canBeFalse)
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.NullableBranch, conditional.GetLocation(), conditional.ToString()));
        }
    }

    private static ExpressionSyntax Unwrapped(ExpressionSyntax expression)
    {
        while (expression is ParenthesizedExpressionSyntax parenthesized)
        {
            expression = parenthesized.Expression;
        }

        return expression;
    }

    // The plain value a nullable branch is made from, (int?)i or new int?(i), which is never nil; null for any other branch.
    private static ExpressionSyntax? Plain(SemanticModel model, ExpressionSyntax branch, SyntaxNodeAnalysisContext context)
    {
        var operand = branch switch
        {
            CastExpressionSyntax cast => cast.Expression,
            ObjectCreationExpressionSyntax { ArgumentList.Arguments.Count: 1 } creation => creation.ArgumentList.Arguments[0].Expression,
            _ => null,
        };

        if (operand == null)
        {
            return null;
        }

        var type = model.GetTypeInfo(operand, context.CancellationToken).Type;
        return type is { IsValueType: true } && type.OriginalDefinition.SpecialType != SpecialType.System_Nullable_T ? operand : null;
    }

    // A value that can be false in the game: a bool, or a type parameter's, which can be one.
    private static bool MayBeBool(ITypeSymbol type)
    {
        return type.SpecialType == SpecialType.System_Boolean || type is ITypeParameterSymbol;
    }

    // n != null, null != n, n.HasValue, n is not null, n is { } or n is int, alone or as a part of an &&: the branch n is not nil when chosen.
    private static bool Checked(SemanticModel model, ExpressionSyntax condition, ExpressionSyntax branch, SyntaxNodeAnalysisContext context)
    {
        if (!Stable(branch))
        {
            return false;
        }

        condition = Unwrapped(condition);
        switch (condition)
        {
            case BinaryExpressionSyntax both when both.IsKind(SyntaxKind.LogicalAndExpression):
                return Checked(model, both.Left, branch, context) || Checked(model, both.Right, branch, context);
            case BinaryExpressionSyntax notEqual when notEqual.IsKind(SyntaxKind.NotEqualsExpression):
                return (Same(notEqual.Left, branch) && IsNullLiteral(notEqual.Right)) || (Same(notEqual.Right, branch) && IsNullLiteral(notEqual.Left));
            case MemberAccessExpressionSyntax { Name.Identifier.ValueText: "HasValue" } access:
                return Same(access.Expression, branch)
                    && model.GetTypeInfo(access.Expression, context.CancellationToken).Type?.OriginalDefinition.SpecialType == SpecialType.System_Nullable_T;
            case BinaryExpressionSyntax isType when isType.IsKind(SyntaxKind.IsExpression):
                return Same(isType.Left, branch);
            case IsPatternExpressionSyntax test:
                return Same(test.Expression, branch) && test.Pattern switch
                {
                    UnaryPatternSyntax { Pattern: ConstantPatternSyntax constant } negated => negated.IsKind(SyntaxKind.NotPattern) && IsNullLiteral(constant.Expression),
                    RecursivePatternSyntax { Type: null, PositionalPatternClause: null, PropertyPatternClause.Subpatterns.Count: 0 } => true,
                    DeclarationPatternSyntax or TypePatternSyntax => true,
                    _ => false,
                };
            default:
                return false;
        }
    }

    // A name or a chain of names, which reads the same value twice in a row; not a call.
    private static bool Stable(ExpressionSyntax expression)
    {
        return Unwrapped(expression) switch
        {
            IdentifierNameSyntax => true,
            MemberAccessExpressionSyntax access => access.Expression is ThisExpressionSyntax or BaseExpressionSyntax || Stable(access.Expression),
            _ => false,
        };
    }

    private static bool Same(ExpressionSyntax a, ExpressionSyntax b)
    {
        return SyntaxFactory.AreEquivalent(Unwrapped(a), Unwrapped(b));
    }

    private static bool IsNullLiteral(ExpressionSyntax expression)
    {
        return Unwrapped(expression).IsKind(SyntaxKind.NullLiteralExpression);
    }

    // null, default, default(int?) or (int?)null: nil in the game.
    private static bool IsNull(SemanticModel model, ExpressionSyntax expression, SyntaxNodeAnalysisContext context)
    {
        expression = Unwrapped(expression);
        if (expression is CastExpressionSyntax cast)
        {
            expression = Unwrapped(cast.Expression);
        }

        if (expression.IsKind(SyntaxKind.NullLiteralExpression))
        {
            return true;
        }

        if (expression.IsKind(SyntaxKind.DefaultLiteralExpression) || expression is DefaultExpressionSyntax)
        {
            // default(int) is 0, made an int? only after.
            return model.GetTypeInfo(expression, context.CancellationToken).Type?.OriginalDefinition.SpecialType == SpecialType.System_Nullable_T;
        }

        return false;
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
