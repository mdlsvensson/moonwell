using Moonwell;
using Moonwell.Objects;

namespace MoonwellMap.Heroes
{
    /// <summary>A Storm Bolt with damage of its own, which tells the caster's owner whom it was thrown at.</summary>
    [ObjectId("A001")]
    public sealed class Smite : Human.Abilities.StormBolt
    {
        public Smite()
        {
            Name = "Smite";
            Damage = Levels(75f, 150f, 225f);
            Tooltip = PerLevel(level => $"Smite - [Level {level}]");
            TooltipExtended = PerLevel(level => $"Deals {Damage[level]} damage and stuns the target.");
            TooltipLearn = "Learn Smite - [Level %d]";
            TooltipLearnExtended = "Throws a hammer at an enemy, which deals more damage at each level and stuns the target.";
        }

        public override void OnCast(Cast cast)
        {
            // A unit's name is in each player's own language, so it is read on the owner's machine only.
            Local.For(cast.Caster.Owner, () =>
            {
                var target = cast.Target == null ? "nothing" : cast.Target.Name;
                cast.Caster.Owner.Print("Smite, level " + cast.Level + ", at " + target);
            });
        }
    }
}
