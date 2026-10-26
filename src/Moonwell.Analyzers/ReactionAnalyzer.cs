using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Diagnostics;

namespace Moonwell.Analyzers;

/// <summary>
/// MW0036: a reaction the package would never call: the damage package's (IDamageSource, IDamageTarget, IDamageWatch) and
/// the physics package's (IPushTarget, IMissileTarget). The package's interfaces give every reaction an empty
/// default body, so the compiler takes a misspelt, non-public or wrongly typed reaction without a word, while the
/// package finds a class's reactions by name and calls only the interface's own. It does nothing for a package the
/// map does not reference.
/// </summary>
[DiagnosticAnalyzer(LanguageNames.CSharp)]
public sealed class ReactionAnalyzer : DiagnosticAnalyzer
{
    /// <summary>The names Moonwell 0.3 gave the reactions it renamed, by interface.</summary>
    private static readonly ImmutableDictionary<string, ImmutableDictionary<string, string>> Renamed =
        ImmutableDictionary.CreateRange(new[]
        {
            new KeyValuePair<string, ImmutableDictionary<string, string>>("IDamageSource", ImmutableDictionary<string, string>.Empty),
            new KeyValuePair<string, ImmutableDictionary<string, string>>("IDamageTarget", ImmutableDictionary.CreateRange(new[]
            {
                new KeyValuePair<string, string>("OnLethal", "OnDying"),
            })),
            new KeyValuePair<string, ImmutableDictionary<string, string>>("IDamageWatch", ImmutableDictionary.CreateRange(new[]
            {
                new KeyValuePair<string, string>("OnBeforeArmor", "OnUnitTaking"),
                new KeyValuePair<string, string>("OnAfterArmor", "OnUnitTaken"),
                new KeyValuePair<string, string>("OnLethal", "OnUnitDying"),
                new KeyValuePair<string, string>("OnLanded", "OnUnitHurt"),
                new KeyValuePair<string, string>("OnBurst", "OnUnitBurst"),
            })),
            // OnLanded is IDamageSource's name: a push's landing is OnGrounded.
            new KeyValuePair<string, ImmutableDictionary<string, string>>("IPushTarget", ImmutableDictionary.CreateRange(new[]
            {
                new KeyValuePair<string, string>("OnLanded", "OnGrounded"),
            })),
            new KeyValuePair<string, ImmutableDictionary<string, string>>("IMissileTarget", ImmutableDictionary<string, string>.Empty),
        });

    public override ImmutableArray<DiagnosticDescriptor> SupportedDiagnostics => Rules.Reactions;

    public override void Initialize(AnalysisContext context)
    {
        context.ConfigureGeneratedCodeAnalysis(GeneratedCodeAnalysisFlags.None);
        context.EnableConcurrentExecution();
        context.RegisterCompilationStartAction(start =>
        {
            var compilation = start.Compilation;
            var source = compilation.GetTypeByMetadataName("Moonwell.IDamageSource");
            var target = compilation.GetTypeByMetadataName("Moonwell.IDamageTarget");
            var watch = compilation.GetTypeByMetadataName("Moonwell.IDamageWatch");
            var pushTarget = compilation.GetTypeByMetadataName("Moonwell.IPushTarget");
            var missileTarget = compilation.GetTypeByMetadataName("Moonwell.IMissileTarget");
            var unit = compilation.GetTypeByMetadataName("Moonwell.Unit");
            var unitComponent = compilation.GetTypeByMetadataName("Moonwell.UnitComponent");
            var mapComponent = compilation.GetTypeByMetadataName("Moonwell.MapComponent");
            if (unit == null || unitComponent == null || mapComponent == null)
            {
                return;
            }

            // A package the map does not reference has no interfaces to check.
            var interfaces = ImmutableArray.CreateRange(new[] { source, target, watch, pushTarget, missileTarget }.Where(i => i != null).Select(i => i!));
            if (interfaces.IsEmpty)
            {
                return;
            }

            var checker = new Checker(interfaces, source, target, watch, unit, unitComponent, mapComponent);
            start.RegisterSymbolAction(checker.Check, SymbolKind.NamedType);
        });
    }

    /// <summary>The checks of one compilation, with the package's types found once.</summary>
    private sealed class Checker
    {
        private readonly INamedTypeSymbol? source;
        private readonly INamedTypeSymbol? target;
        private readonly INamedTypeSymbol? watch;
        private readonly INamedTypeSymbol unit;
        private readonly INamedTypeSymbol unitComponent;
        private readonly INamedTypeSymbol mapComponent;
        private readonly ImmutableArray<INamedTypeSymbol> interfaces;

        public Checker(ImmutableArray<INamedTypeSymbol> interfaces, INamedTypeSymbol? source, INamedTypeSymbol? target, INamedTypeSymbol? watch, INamedTypeSymbol unit, INamedTypeSymbol unitComponent, INamedTypeSymbol mapComponent)
        {
            this.source = source;
            this.target = target;
            this.watch = watch;
            this.unit = unit;
            this.unitComponent = unitComponent;
            this.mapComponent = mapComponent;
            this.interfaces = interfaces;
        }

        /// <summary>Checks a class that implements a reaction interface, or is a unit or a component (which might have forgotten one).</summary>
        public void Check(SymbolAnalysisContext context)
        {
            if (context.Symbol is not INamedTypeSymbol { TypeKind: TypeKind.Class } type)
            {
                return;
            }

            var implemented = interfaces.Where(i => type.AllInterfaces.Contains(i, SymbolEqualityComparer.Default)).ToList();
            if (implemented.Count == 0 && !IsA(type, unit) && !IsA(type, unitComponent) && !IsA(type, mapComponent))
            {
                return;
            }

            CheckHosts(context, type);
            foreach (var method in type.GetMembers().OfType<IMethodSymbol>())
            {
                if (method.IsImplicitlyDeclared || method.IsOverride || !method.Locations.Any(location => location.IsInSource))
                {
                    continue;
                }

                if (method.MethodKind == MethodKind.ExplicitInterfaceImplementation)
                {
                    var reaction = method.ExplicitInterfaceImplementations.FirstOrDefault(m => interfaces.Contains(m.ContainingType, SymbolEqualityComparer.Default));
                    if (reaction != null)
                    {
                        Report(context, method, reaction.Name, "never runs: the package calls only public methods, so write it as public void " + reaction.Name + "(...)");
                    }

                    continue;
                }

                if (method.MethodKind != MethodKind.Ordinary || Implements(type, implemented, method))
                {
                    continue;
                }

                var why = Why(type, method, implemented);
                if (why != null)
                {
                    Report(context, method, method.Name, why);
                }
            }
        }

        /// <summary>Why a method that implements no reaction looks like one; null when it does not.</summary>
        private string? Why(INamedTypeSymbol type, IMethodSymbol method, List<INamedTypeSymbol> implemented)
        {
            var name = method.Name;
            foreach (var i in implemented)
            {
                var reaction = Reactions(i).FirstOrDefault(r => r.Name == name);
                if (reaction == null)
                {
                    continue;
                }

                // Written right, yet not the implementation: the class gets the interface from a base class, whose mapping
                // it keeps (the game's Lua calls it by name, .NET through the interface).
                if (method.DeclaredAccessibility == Accessibility.Public && !method.IsStatic && method.Parameters.Length == 1
                    && SymbolEqualityComparer.Default.Equals(method.Parameters[0].Type, reaction.Parameters[0].Type))
                {
                    return "is not " + i.Name + "'s reaction here, as " + type.Name + " gets " + i.Name + " from a base class: list " + i.Name + " among " + type.Name + "'s own bases too";
                }

                return "never runs: a reaction of " + i.Name + " is public, not static, and takes a " + reaction.Parameters[0].Type.ToDisplayString(SymbolDisplayFormat.MinimallyQualifiedFormat);
            }

            // The rest is only for methods shaped like a reaction: one parameter, a Hit, Burst, Impact or Slam, or the type
            // the reaction in question takes.
            if (method.Parameters.Length != 1)
            {
                return null;
            }

            var renamed = implemented
                .Where(i => Renamed.TryGetValue(i.Name, out var names) && names.TryGetValue(name, out var now)
                    && Shaped(method, Reactions(i).FirstOrDefault(r => r.Name == now)))
                .ToList();
            if (renamed.Count > 0)
            {
                // Only the damage package's names changed in 0.3; the physics package began in it.
                var since = renamed.All(i => i.Name.StartsWith("IDamage", StringComparison.Ordinal)) ? " since Moonwell 0.3" : string.Empty;
                return "never runs: " + string.Join(", and ", renamed.Select(i => i.Name + "'s reaction of that stage is " + Renamed[i.Name][name])) + since;
            }

            // A forgotten interface; an abstract class may be writing reactions for subclasses that list it.
            if (!type.IsAbstract)
            {
                foreach (var i in interfaces)
                {
                    if (Reactions(i).Any(r => r.Name == name && Shaped(method, r)))
                    {
                        return "never runs: it is a reaction of " + i.Name + ", which this class does not implement";
                    }
                }
            }

            // A one-letter slip (a letter missing, extra, changed or two swapped) in a public method of a class that has
            // reactions; a helper of another name, or a private one, is left alone.
            if (implemented.Count == 0 || method.DeclaredAccessibility != Accessibility.Public)
            {
                return null;
            }

            var near = implemented
                .SelectMany(i => Reactions(i).Where(r => Shaped(method, r)).Select(r => (Interface: i, Name: r.Name, Distance: Distance(name, r.Name))))
                .OrderBy(candidate => candidate.Distance)
                .FirstOrDefault();
            if (near.Interface != null && near.Distance <= 1)
            {
                return "is not a reaction of " + near.Interface.Name + ", so it never runs; did you mean " + near.Name + "?";
            }

            return null;
        }

        /// <summary>Reports a damage interface on a class the package never calls it on.</summary>
        private void CheckHosts(SymbolAnalysisContext context, INamedTypeSymbol type)
        {
            foreach (var i in type.Interfaces)
            {
                string? hosts = null;
                if (!interfaces.Contains(i, SymbolEqualityComparer.Default))
                {
                    continue;
                }

                if (!SymbolEqualityComparer.Default.Equals(i, watch))
                {
                    // Every other reaction interface (damage source and target, push and missile targets) is called on a unit's
                    // class and its components.
                    if (!IsA(type, unit) && !IsA(type, unitComponent))
                    {
                        hosts = "a unit's class and its components (buffs too)";
                    }
                }
                else if (!IsA(type, mapComponent))
                {
                    hosts = "the game's and players' components";
                }

                if (hosts == null)
                {
                    continue;
                }

                var location = BaseTypeLocation(type, i) ?? type.Locations[0];
                context.ReportDiagnostic(Diagnostic.Create(Rules.UnseenReaction, location, i.Name, "on this class never runs: the " + PackageOf(i) + " package calls " + i.Name + " only on " + hosts));
            }
        }

        /// <summary>Whether the method is the class's implementation of a reaction of an interface it implements.</summary>
        private static bool Implements(INamedTypeSymbol type, List<INamedTypeSymbol> implemented, IMethodSymbol method)
        {
            foreach (var i in implemented)
            {
                foreach (var reaction in Reactions(i))
                {
                    if (SymbolEqualityComparer.Default.Equals(type.FindImplementationForInterfaceMember(reaction), method))
                    {
                        return true;
                    }
                }
            }

            return false;
        }

        /// <summary>Whether a type is one a reaction takes: the damage package's Hit or Burst, or the physics package's Impact or Slam.</summary>
        private static bool IsReactionParameter(ITypeSymbol type)
        {
            return type.ContainingNamespace?.ToDisplayString() == "Moonwell" && (type.Name == "Hit" || type.Name == "Burst" || type.Name == "Impact" || type.Name == "Slam");
        }

        /// <summary>Whether a one-parameter method is shaped like a reaction: its parameter is a type reactions take, or the type this one takes.</summary>
        private static bool Shaped(IMethodSymbol method, IMethodSymbol? reaction)
        {
            var parameter = method.Parameters[0].Type;
            return IsReactionParameter(parameter)
                || (reaction != null && SymbolEqualityComparer.Default.Equals(parameter, reaction.Parameters[0].Type));
        }

        /// <summary>The name of the package an interface is from, for the message.</summary>
        private static string PackageOf(INamedTypeSymbol i)
        {
            return i.Name == "IPushTarget" || i.Name == "IMissileTarget" ? "physics" : "damage";
        }

        /// <summary>The reaction methods of a damage interface.</summary>
        private static IEnumerable<IMethodSymbol> Reactions(INamedTypeSymbol i)
        {
            return i.GetMembers().OfType<IMethodSymbol>().Where(m => m.MethodKind == MethodKind.Ordinary && m.Parameters.Length == 1);
        }

        /// <summary>Whether a type is a class or derives from it.</summary>
        private static bool IsA(INamedTypeSymbol type, INamedTypeSymbol baseType)
        {
            for (INamedTypeSymbol? current = type; current != null; current = current.BaseType)
            {
                if (SymbolEqualityComparer.Default.Equals(current.OriginalDefinition, baseType))
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>Where the class lists an interface among its bases.</summary>
        private static Location? BaseTypeLocation(INamedTypeSymbol type, INamedTypeSymbol i)
        {
            foreach (var reference in type.DeclaringSyntaxReferences)
            {
                if (reference.GetSyntax() is ClassDeclarationSyntax { BaseList: { } bases })
                {
                    var written = bases.Types.FirstOrDefault(b => b.Type.ToString() == i.Name || b.Type.ToString().EndsWith("." + i.Name, StringComparison.Ordinal));
                    if (written != null)
                    {
                        return written.Type.GetLocation();
                    }
                }
            }

            return null;
        }

        /// <summary>The number of slips that turn one name into another: a letter added, removed or changed, or two next to each other swapped.</summary>
        private static int Distance(string a, string b)
        {
            var d = new int[a.Length + 1, b.Length + 1];
            for (var i = 0; i <= a.Length; i++)
            {
                d[i, 0] = i;
            }

            for (var j = 0; j <= b.Length; j++)
            {
                d[0, j] = j;
            }

            for (var i = 1; i <= a.Length; i++)
            {
                for (var j = 1; j <= b.Length; j++)
                {
                    var change = a[i - 1] == b[j - 1] ? 0 : 1;
                    d[i, j] = Math.Min(Math.Min(d[i - 1, j] + 1, d[i, j - 1] + 1), d[i - 1, j - 1] + change);
                    if (i > 1 && j > 1 && a[i - 1] == b[j - 2] && a[i - 2] == b[j - 1])
                    {
                        d[i, j] = Math.Min(d[i, j], d[i - 2, j - 2] + 1);
                    }
                }
            }

            return d[a.Length, b.Length];
        }

        private static void Report(SymbolAnalysisContext context, IMethodSymbol method, string name, string why)
        {
            context.ReportDiagnostic(Diagnostic.Create(Rules.UnseenReaction, method.Locations[0], name, why));
        }
    }
}
