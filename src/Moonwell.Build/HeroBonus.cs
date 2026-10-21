namespace Moonwell.Build
{
    /// <summary>What a hero's attributes add to its life and mana, from the gameplay constants.</summary>
    /// <param name="LifePerStrength">The life each point of strength adds (<c>StrHitPointBonus</c>).</param>
    /// <param name="ManaPerIntelligence">The mana each point of intelligence adds (<c>IntManaBonus</c>).</param>
    public sealed record HeroBonus(float LifePerStrength, float ManaPerIntelligence);
}
