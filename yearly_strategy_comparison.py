#!/usr/bin/env python3
"""
Yearly Strategy Performance Comparison

Creates a comprehensive yearly breakdown of all trading strategies
"""

def create_yearly_comparison():
    """Create yearly comparison table"""
    
    # Manual data from database queries and spy analysis
    spy_data = {
        2020: 16.24,
        2021: 30.51, 
        2022: -18.65,
        2023: 26.71,
        2024: 25.59,
        2025: 0.68  # partial year
    }
    
    # Original strategy data
    original_data = {
        2020: 40.6,
        2021: 76.55,
        2022: -26.5,
        2023: -19.71,
        2024: 43.75,
        2025: -8.19
    }
    
    # Confidence strategy data
    confidence_data = {
        2020: 55.06,
        2021: 32.18,
        2022: -34.57,
        2023: 18.45,
        2024: 28.5,
        2025: 16.24
    }
    
    print("=" * 120)
    print("YEARLY STRATEGY PERFORMANCE COMPARISON")
    print("=" * 120)
    print(f"{'Year':<6} {'SPY':<10} {'Original':<12} {'Confidence':<12} {'Orig vs SPY':<12} {'Conf vs SPY':<12} {'Conf vs Orig':<12}")
    print("-" * 120)
    
    total_spy = 100000
    total_orig = 100000
    total_conf = 100000
    
    for year in [2020, 2021, 2022, 2023, 2024, 2025]:
        spy_ret = spy_data[year]
        orig_ret = original_data[year]
        conf_ret = confidence_data[year]
        
        # Calculate outperformance
        orig_vs_spy = orig_ret - spy_ret
        conf_vs_spy = conf_ret - spy_ret
        conf_vs_orig = conf_ret - orig_ret
        
        # Update cumulative values
        total_spy *= (1 + spy_ret/100)
        total_orig *= (1 + orig_ret/100)  
        total_conf *= (1 + conf_ret/100)
        
        print(f"{year:<6} {spy_ret:+7.1f}%  {orig_ret:+9.1f}%  {conf_ret:+9.1f}%  {orig_vs_spy:+9.1f}%  {conf_vs_spy:+9.1f}%  {conf_vs_orig:+9.1f}%")
    
    print("-" * 120)
    
    # Calculate cumulative returns
    spy_total_ret = (total_spy/100000 - 1) * 100
    orig_total_ret = (total_orig/100000 - 1) * 100
    conf_total_ret = (total_conf/100000 - 1) * 100
    
    print(f"{'TOTAL':<6} {spy_total_ret:+7.1f}%  {orig_total_ret:+9.1f}%  {conf_total_ret:+9.1f}%  {orig_total_ret-spy_total_ret:+9.1f}%  {conf_total_ret-spy_total_ret:+9.1f}%  {conf_total_ret-orig_total_ret:+9.1f}%")
    print(f"{'VALUE':<6} ${total_spy:>8,.0f} ${total_orig:>10,.0f} ${total_conf:>10,.0f}")
    
    print("=" * 120)
    
    # Year-by-year analysis
    print("\nYEAR-BY-YEAR ANALYSIS:")
    print("-" * 60)
    
    best_years = {"spy": [], "orig": [], "conf": []}
    worst_years = {"spy": [], "orig": [], "conf": []}
    
    for year in [2020, 2021, 2022, 2023, 2024]:  # Exclude partial 2025
        spy_ret = spy_data[year]
        orig_ret = original_data[year]
        conf_ret = confidence_data[year]
        
        if spy_ret > 20:
            best_years["spy"].append(year)
        elif spy_ret < -10:
            worst_years["spy"].append(year)
            
        if orig_ret > 20:
            best_years["orig"].append(year)
        elif orig_ret < -10:
            worst_years["orig"].append(year)
            
        if conf_ret > 20:
            best_years["conf"].append(year)
        elif conf_ret < -10:
            worst_years["conf"].append(year)
    
    print("STRONG PERFORMANCE YEARS (>20%):")
    print(f"  SPY: {best_years['spy']} ({len(best_years['spy'])}/5 years)")
    print(f"  Original Strategy: {best_years['orig']} ({len(best_years['orig'])}/5 years)")
    print(f"  Confidence Strategy: {best_years['conf']} ({len(best_years['conf'])}/5 years)")
    
    print("\nPOOR PERFORMANCE YEARS (<-10%):")
    print(f"  SPY: {worst_years['spy']} ({len(worst_years['spy'])}/5 years)")
    print(f"  Original Strategy: {worst_years['orig']} ({len(worst_years['orig'])}/5 years)")
    print(f"  Confidence Strategy: {worst_years['conf']} ({len(worst_years['conf'])}/5 years)")
    
    # Volatility analysis
    spy_returns = [spy_data[y] for y in [2020, 2021, 2022, 2023, 2024]]
    orig_returns = [original_data[y] for y in [2020, 2021, 2022, 2023, 2024]]
    conf_returns = [confidence_data[y] for y in [2020, 2021, 2022, 2023, 2024]]
    
    import statistics
    
    print(f"\nVOLATILITY ANALYSIS (Standard Deviation of Annual Returns):")
    print(f"  SPY: {statistics.stdev(spy_returns):.1f}%")
    print(f"  Original Strategy: {statistics.stdev(orig_returns):.1f}%")
    print(f"  Confidence Strategy: {statistics.stdev(conf_returns):.1f}%")
    
    print(f"\nCONSISTENCY (% of years with positive returns):")
    spy_positive = sum(1 for x in spy_returns if x > 0) / len(spy_returns) * 100
    orig_positive = sum(1 for x in orig_returns if x > 0) / len(orig_returns) * 100
    conf_positive = sum(1 for x in conf_returns if x > 0) / len(conf_returns) * 100
    
    print(f"  SPY: {spy_positive:.0f}%")
    print(f"  Original Strategy: {orig_positive:.0f}%")
    print(f"  Confidence Strategy: {conf_positive:.0f}%")

if __name__ == "__main__":
    create_yearly_comparison()