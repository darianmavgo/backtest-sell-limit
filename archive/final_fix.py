#!/usr/bin/env python3

def main():
    with open('main.go', 'r') as f:
        lines = f.readlines()
    
    output_lines = []
    i = 0
    
    while i < len(lines):
        line = lines[i]
        
        # Add the getActiveSP500Tickers function before fetchStockData
        if '// fetchStockData fetches stock data for a given ticker using a free API' in line:
            # Add our new function
            output_lines.extend([
                '// getActiveSP500Tickers fetches active ticker symbols from the database\n',
                'func (db *DB) getActiveSP500Tickers() ([]string, error) {\n',
                '\tquery := "SELECT ticker FROM sp500_list_2025_jun WHERE is_active = 1 ORDER BY ticker"\n',
                '\trows, err := db.Query(query)\n',
                '\tif err != nil {\n',
                '\t\treturn nil, fmt.Errorf("failed to query tickers: %v", err)\n',
                '\t}\n',
                '\tdefer rows.Close()\n',
                '\n',
                '\tvar tickers []string\n',
                '\tfor rows.Next() {\n',
                '\t\tvar ticker string\n',
                '\t\tif err := rows.Scan(&ticker); err != nil {\n',
                '\t\t\treturn nil, fmt.Errorf("failed to scan ticker: %v", err)\n',
                '\t\t}\n',
                '\t\ttickers = append(tickers, ticker)\n',
                '\t}\n',
                '\n',
                '\tif err := rows.Err(); err != nil {\n',
                '\t\treturn nil, fmt.Errorf("row iteration error: %v", err)\n',
                '\t}\n',
                '\n',
                '\treturn tickers, nil\n',
                '}\n',
                '\n'
            ])
            # Add the original fetchStockData line
            output_lines.append(line)
        
        # Update the fetchAllSP500Data function start
        elif 'func fetchAllSP500Data(db *DB) error {' in line:
            output_lines.append(line)
            i += 1
            # Add database ticker loading
            output_lines.extend([
                '\t// Get ticker list from database\n',
                '\ttickers, err := db.getActiveSP500Tickers()\n',
                '\tif err != nil {\n',
                '\t\treturn fmt.Errorf("failed to get ticker list: %v", err)\n',
                '\t}\n',
                '\n',
                '\tif len(tickers) == 0 {\n',
                '\t\treturn fmt.Errorf("no active tickers found in database")\n',
                '\t}\n',
                '\n'
            ])
            # Skip the original numWorkers line and handle it
            while i < len(lines) and 'numWorkers := 20' not in lines[i]:
                i += 1
            # Add the numWorkers line and continue
            if i < len(lines):
                output_lines.append(lines[i])
        
        # Replace sp500Tickers references
        elif 'len(sp500Tickers)' in line:
            output_lines.append(line.replace('len(sp500Tickers)', 'len(tickers)'))
        elif 'for _, ticker := range sp500Tickers {' in line:
            output_lines.append(line.replace('sp500Tickers', 'tickers'))
        else:
            output_lines.append(line)
        
        i += 1
    
    # Write the result
    with open('main.go', 'w') as f:
        f.writelines(output_lines)
    
    print("✅ Successfully converted to database-driven ticker loading!")

if __name__ == "__main__":
    main()
